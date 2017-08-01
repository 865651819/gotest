package main

import (
	"bytes"
	vdao "code.byted.org/videoarch/transcoder/thrift_gen/toutiao/videoarch/video_data_access"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type OFMetric struct {
	Endpoint string `json:"endpoint"`
	Counter  string `json:"counter"`
}

type OFRequest struct {
	Start            int64      `json:"start"`
	End              int64      `json:"end"`
	Cf               string     `json:"cf"`
	EndpointCounters []OFMetric `json:"endpoint_counters"`
}

type RSPMetric struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type OFResponse struct {
	Endpoint string      `json:"endpoint"`
	Counter  string      `json:"counter"`
	Dstype   string      `json:"dstype"`
	Step     int32       `json:"step"`
	Values   []RSPMetric `json:"Values"`
}

type ClusterInfo struct {
	Users            []string
	QuotaPct         []float64
	TcronCluster     string
	CpuUsed          float64
	AvailableLoadPct []float64
	IncreasedLoad    []float64
}

type DAGConf struct {
	UserPriority [][]string `json:"user_priority"`

	Clusters map[string]struct {
		EnvsRaw  map[string]*json.RawMessage `json:"envs"`
		Users    []string                    `json:"users"`
		QuotaPct []float64                   `json:"quota_pct"`
	} `json:"clusters"`

	Dags []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Load    int    `json:"load"`
	} `json:"dags"`
}

const (
	END_POINT                    = "n6-193-161"
	COUNTER_PREFIX               = "cpu.idle.arv/ams_tag=web.videoarch.tasks."
	OPEN_FALCON_HOST             = "http://query.d.byted.org:9966/graph/history"
	OPEN_FALCON_QUERY_TIME_RANGE = 130
	REFRESH_INTERVAL             = 30
	DEFAULT_VIDEO_LOAD           = 1000 * 1000 * 3600
)

var (
	clusterInfos     map[string]*ClusterInfo = make(map[string]*ClusterInfo)
	refreshTimestamp int64
	lowestPriority   int                       = -1
	workFlowLoadMap  map[string]map[string]int = make(map[string]map[string]int)
	userPriorityMap  map[string]int            = make(map[string]int)
	testClusterNames map[string]bool           = map[string]bool{"test": true}
	tcronClusterMap  map[string]string         = make(map[string]string)
)

func (clusterInfo *ClusterInfo) String() (retStr string) {
	retStr += "{ "
	retStr += fmt.Sprintf("Users: %v | ", clusterInfo.Users)
	retStr += fmt.Sprintf("QuotaPct: %v | ", clusterInfo.QuotaPct)
	retStr += fmt.Sprintf("TcronCluster: %v | ", clusterInfo.TcronCluster)
	retStr += fmt.Sprintf("CpuUsed: %v | ", clusterInfo.CpuUsed)
	retStr += fmt.Sprintf("AvailableLoadPct: %v | ", clusterInfo.AvailableLoadPct)
	retStr += fmt.Sprintf("IncreasedLoad: %v", clusterInfo.IncreasedLoad)
	retStr += " }"
	return
}

func init() {
	initClusterInfoFromJson()

	fmt.Println("clusterInfos:")
	for k, v := range clusterInfos {
		fmt.Println(k, ":", v)
	}
	fmt.Println()
	fmt.Println("lowestPriority: ", lowestPriority)
	fmt.Println()
	fmt.Println("tcronClusterMap: ", tcronClusterMap)
	fmt.Println()
	fmt.Printf("workFlowLoadMap: (len %d)\n", len(workFlowLoadMap))
	for k, v := range workFlowLoadMap {
		for kk, vv := range v {
			fmt.Println(k, kk, vv)
		}
	}
	fmt.Println()
	fmt.Println("userPriorityMap:")
	for k, v := range userPriorityMap {
		fmt.Println(k, ":", v)
	}
	fmt.Println()
}

func initClusterInfoFromJson() {
	raw, err := ioutil.ReadFile("/Users/zhangyi/go/src/gotest/open-falcon/dag_test.json")
	if err != nil {
		panic(err)
	}
	var conf DAGConf
	err = json.Unmarshal(raw, &conf)
	if err != nil {
		panic(err)
	}

	for cluster, clusterConf := range conf.Clusters {
		if testClusterNames[cluster] {
			continue
		}
		clusterInfos[cluster] = new(ClusterInfo)
		clusterInfos[cluster].Users = clusterConf.Users
		tcronCluster := string(*(clusterConf.EnvsRaw["tcron_cluster"]))
		tcronCluster = strings.Trim(tcronCluster, "\"")
		clusterInfos[cluster].TcronCluster = tcronCluster
		tcronClusterMap[tcronCluster] = cluster
		clusterInfos[cluster].QuotaPct = clusterConf.QuotaPct

		quota_pct_len := len(clusterConf.QuotaPct)
		if quota_pct_len <= 0 {
			log.Panicf("empty quota_pct in cluster: %s \n", cluster)
		}
		if lowestPriority == -1 {
			lowestPriority = len(clusterConf.QuotaPct) - 1
		} else if lowestPriority != len(clusterConf.QuotaPct)-1 {
			log.Panicln("unequal size for quota_pct in each cluster")
		}
		clusterInfos[cluster].AvailableLoadPct = make([]float64, lowestPriority+1)
		clusterInfos[cluster].IncreasedLoad = make([]float64, lowestPriority+1)
	}

	for _, dag := range conf.Dags {
		if workFlowLoadMap[dag.Name] == nil {
			workFlowLoadMap[dag.Name] = make(map[string]int)
		}
		workFlowLoadMap[dag.Name][dag.Version] = dag.Load
	}
	for i, userList := range conf.UserPriority {
		for _, user := range userList {
			userPriorityMap[user] = i
		}
	}
}

func updateCpuUsed() error {
	if len(clusterInfos) == 0 {
		return errors.New("empty clusterInfos")
	}
	ofMetrics := make([]OFMetric, 0, len(clusterInfos))
	for _, clusterInfo := range clusterInfos {
		ofMetrics = append(ofMetrics, OFMetric{Endpoint: END_POINT, Counter: COUNTER_PREFIX + clusterInfo.TcronCluster})
	}
	ofRequest := OFRequest{Start: time.Now().Unix() - OPEN_FALCON_QUERY_TIME_RANGE,
		End: time.Now().Unix(), Cf: "AVERAGE", EndpointCounters: ofMetrics}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(ofRequest)
	fmt.Println("Open-falcon request:")
	fmt.Println(b)
	res, err := http.Post(OPEN_FALCON_HOST, "application/json; charset=utf-8", b)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return errors.New("open-falcon returns error, status: " + res.Status)
	}
	var body []OFResponse
	json.NewDecoder(res.Body).Decode(&body)

	if len(body) != len(clusterInfos) {
		return errors.New("open-falcon not match clusterInfos")
	}
	fmt.Println("Open-falcon response:")
	for i := 0; i < len(body); i++ {
		fmt.Printf("%s ", body[i].Counter)
		for j := 0; j < len(body[i].Values); j++ {
			fmt.Printf("%f ", body[i].Values[j].Value)
		}
		fmt.Println()
	}
	fmt.Println()
	for i := 0; i < len(body); i++ {
		for j := len(body[i].Values) - 1; j >= 0; j-- {
			if body[i].Values[j].Value > 0.0 {
				tcronClusterName := strings.TrimPrefix(body[i].Counter, COUNTER_PREFIX)
				clusterName := tcronClusterMap[tcronClusterName]
				clusterInfos[clusterName].CpuUsed = 100 - body[i].Values[j].Value
				break
			}
		}

	}
	return nil
}

func refreshClusterInfos() error {
	err := updateCpuUsed()
	if err != nil {
		return err
	}
	for _, clusterInfo := range clusterInfos {
		for i, v := range clusterInfo.QuotaPct {
			if clusterInfo.CpuUsed >= v {
				clusterInfo.AvailableLoadPct[i] = 0
			} else {
				clusterInfo.AvailableLoadPct[i] = v - clusterInfo.CpuUsed
			}
		}
		for i := range clusterInfo.IncreasedLoad {
			clusterInfo.IncreasedLoad[i] = 1.0
		}
	}

	fmt.Println("clusterInfos:")
	for k, v := range clusterInfos {
		fmt.Println(k, ":", v)
	}
	fmt.Println()
	return nil
}

func getLoadByVideoInfo(videoInfo *vdao.VideoGroupInfo) float64 {
	if videoInfo == nil || videoInfo.OriginalVideoInfo == nil || videoInfo.OriginalVideoInfo.MetaInfo == nil {
		return 1
	}
	videoMeta := videoInfo.OriginalVideoInfo.MetaInfo
	return float64(videoMeta.GetHeight()) * float64(videoMeta.GetWidth()) *
		float64(videoMeta.GetDuration()) / DEFAULT_VIDEO_LOAD
}

func GetClusterDynamic(user string, workFlowName string, workFlowVersion string,
	videoInfo *vdao.VideoGroupInfo) (string, error) {
	priority, upmOk := userPriorityMap[user]
	if !upmOk {
		return "", errors.New("invalid user: " + user)
	}
	workFlowLoad := workFlowLoadMap[workFlowName][workFlowVersion]
	if workFlowLoad < 1 {
		return "", errors.New("invalid work flow name:" + workFlowName + " or version:" + workFlowVersion)
	}
	load := getLoadByVideoInfo(videoInfo) * float64(workFlowLoad)
	fmt.Println("workLoad:", load)
	fmt.Println()
	currentTimestamp := time.Now().Unix()
	if currentTimestamp-refreshTimestamp > REFRESH_INTERVAL {
		rciError := refreshClusterInfos()
		if rciError != nil {
			return "", rciError
		}
		refreshTimestamp = currentTimestamp
	}
	if priority > lowestPriority {
		priority = lowestPriority
	}
	var maxAvailableCluster string
	maxAvailableLoad := -1.0
	for clusterName, clusterInfo := range clusterInfos {
		availableLoadPct := clusterInfo.AvailableLoadPct[priority]
		increasedLoad := clusterInfo.IncreasedLoad[priority]
		fmt.Println(clusterName, ": availableLoadPct/increasedLoad:", availableLoadPct/increasedLoad)
		if availableLoadPct/increasedLoad >= maxAvailableLoad {
			maxAvailableLoad = availableLoadPct / increasedLoad
			maxAvailableCluster = clusterName
		}
	}
	fmt.Println("maxAva:", maxAvailableCluster)
	if maxAvailableCluster == "" {
		return "", errors.New("empty maxAvailableCluster")
	}
	clusterInfos[maxAvailableCluster].IncreasedLoad[priority] += load
	return maxAvailableCluster, nil
}

func main() {
	user := "crawl"
	workFlowName := "flipagram"
	workFlowVersion := "1"
	var videoInfo *vdao.VideoGroupInfo
	for i := 0; i < 10000; i++ {
		clusterName, err := GetClusterDynamic(user, workFlowName, workFlowVersion, videoInfo)
		if err != nil {
			fmt.Println("error in GetClusterDynamic: ", err)
		} else {
			fmt.Println("GetClusterDynamic return: ", clusterName)
			fmt.Println("clusterInfos:")
			for k, v := range clusterInfos {
				fmt.Println(k, ":", v)
			}
			fmt.Println()
		}
	}
}
