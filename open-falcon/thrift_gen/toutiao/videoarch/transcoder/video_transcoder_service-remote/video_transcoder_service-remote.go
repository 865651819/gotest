// Autogenerated by Thrift Compiler (0.9.2)
// DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING

package main

import (
	"code.byted.org/gopkg/thrift"
	"flag"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"toutiao/videoarch/transcoder"
)

func Usage() {
	fmt.Fprintln(os.Stderr, "Usage of ", os.Args[0], " [-h host:port] [-u url] [-f[ramed]] function [arg1 [arg2...]]:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "\nFunctions:")
	fmt.Fprintln(os.Stderr, "  StartTranscodeResponse StartTranscode(StartTranscodeRequest request)")
	fmt.Fprintln(os.Stderr, "  StartTaskResponse StartTask(StartTaskRequest request)")
	fmt.Fprintln(os.Stderr)
	os.Exit(0)
}

func main() {
	flag.Usage = Usage
	var host string
	var port int
	var protocol string
	var urlString string
	var framed bool
	var useHttp bool
	var parsedUrl url.URL
	var trans thrift.TTransport
	_ = strconv.Atoi
	_ = math.Abs
	flag.Usage = Usage
	flag.StringVar(&host, "h", "localhost", "Specify host and port")
	flag.IntVar(&port, "p", 9090, "Specify port")
	flag.StringVar(&protocol, "P", "binary", "Specify the protocol (binary, compact, simplejson, json)")
	flag.StringVar(&urlString, "u", "", "Specify the url")
	flag.BoolVar(&framed, "framed", false, "Use framed transport")
	flag.BoolVar(&useHttp, "http", false, "Use http")
	flag.Parse()

	if len(urlString) > 0 {
		parsedUrl, err := url.Parse(urlString)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing URL: ", err)
			flag.Usage()
		}
		host = parsedUrl.Host
		useHttp = len(parsedUrl.Scheme) <= 0 || parsedUrl.Scheme == "http"
	} else if useHttp {
		_, err := url.Parse(fmt.Sprint("http://", host, ":", port))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing URL: ", err)
			flag.Usage()
		}
	}

	cmd := flag.Arg(0)
	var err error
	if useHttp {
		trans, err = thrift.NewTHttpClient(parsedUrl.String())
	} else {
		portStr := fmt.Sprint(port)
		if strings.Contains(host, ":") {
			host, portStr, err = net.SplitHostPort(host)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error with host:", err)
				os.Exit(1)
			}
		}
		trans, err = thrift.NewTSocket(net.JoinHostPort(host, portStr))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error resolving address:", err)
			os.Exit(1)
		}
		if framed {
			trans = thrift.NewTFramedTransport(trans)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating transport", err)
		os.Exit(1)
	}
	defer trans.Close()
	var protocolFactory thrift.TProtocolFactory
	switch protocol {
	case "compact":
		protocolFactory = thrift.NewTCompactProtocolFactory()
		break
	case "simplejson":
		protocolFactory = thrift.NewTSimpleJSONProtocolFactory()
		break
	case "json":
		protocolFactory = thrift.NewTJSONProtocolFactory()
		break
	case "binary", "":
		protocolFactory = thrift.NewTBinaryProtocolFactoryDefault()
		break
	default:
		fmt.Fprintln(os.Stderr, "Invalid protocol specified: ", protocol)
		Usage()
		os.Exit(1)
	}
	client := transcoder.NewVideoTranscoderServiceClientFactory(trans, protocolFactory)
	if err := trans.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "Error opening socket to ", host, ":", port, " ", err)
		os.Exit(1)
	}

	switch cmd {
	case "StartTranscode":
		if flag.NArg()-1 != 1 {
			fmt.Fprintln(os.Stderr, "StartTranscode requires 1 args")
			flag.Usage()
		}
		arg10 := flag.Arg(1)
		mbTrans11 := thrift.NewTMemoryBufferLen(len(arg10))
		defer mbTrans11.Close()
		_, err12 := mbTrans11.WriteString(arg10)
		if err12 != nil {
			Usage()
			return
		}
		factory13 := thrift.NewTSimpleJSONProtocolFactory()
		jsProt14 := factory13.GetProtocol(mbTrans11)
		argvalue0 := transcoder.NewStartTranscodeRequest()
		err15 := argvalue0.Read(jsProt14)
		if err15 != nil {
			Usage()
			return
		}
		value0 := argvalue0
		fmt.Print(client.StartTranscode(value0))
		fmt.Print("\n")
		break
	case "StartTask":
		if flag.NArg()-1 != 1 {
			fmt.Fprintln(os.Stderr, "StartTask requires 1 args")
			flag.Usage()
		}
		arg16 := flag.Arg(1)
		mbTrans17 := thrift.NewTMemoryBufferLen(len(arg16))
		defer mbTrans17.Close()
		_, err18 := mbTrans17.WriteString(arg16)
		if err18 != nil {
			Usage()
			return
		}
		factory19 := thrift.NewTSimpleJSONProtocolFactory()
		jsProt20 := factory19.GetProtocol(mbTrans17)
		argvalue0 := transcoder.NewStartTaskRequest()
		err21 := argvalue0.Read(jsProt20)
		if err21 != nil {
			Usage()
			return
		}
		value0 := argvalue0
		fmt.Print(client.StartTask(value0))
		fmt.Print("\n")
		break
	case "":
		Usage()
		break
	default:
		fmt.Fprintln(os.Stderr, "Invalid function ", cmd)
	}
}
