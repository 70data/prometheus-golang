package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/70data/golang-prometheus/prometheus"
	"github.com/70data/golang-prometheus/prometheus/promhttp"

	_ "github.com/mkevac/debugcharts"
)

// {"app":"sre","metric":"gatewaytest","value":"12345", "timeout":"60"}

var ReqQueue chan string

var (
	LabelStore map[string]int64
	lsSync     sync.Mutex
)

var (
	ValueStore map[string]interface{}
	vsSync     sync.Mutex
)

var (
	TimeOutLabelStore map[string]string
	toLabelSync       sync.Mutex
)

var (
	TimeOutLineStore map[string]int64
	toLineSync       sync.Mutex
)

func mapSort(naiveMap map[string]interface{}) []string {
	sortedKeys := make([]string, 0)
	for indexK := range naiveMap {
		if indexK != "type" && indexK != "metric" && indexK != "value" && indexK != "timeout" {
			sortedKeys = append(sortedKeys, indexK)
		}
	}
	sort.Strings(sortedKeys)
	return sortedKeys
}

func timeOutMark(cur int64, timeOutMap map[string]interface{}) {
	timeOut := timeOutMap["timeout"].(string)
	if timeOut != "" {
		delete(timeOutMap, "value")
		resaultJSON, err := json.Marshal(timeOutMap)
		if err != nil {
			fmt.Println(err)
		}
		resaultBase := base64.StdEncoding.EncodeToString(resaultJSON)
		toLabelSync.Lock()
		TimeOutLabelStore[resaultBase] = timeOut
		toLabelSync.Unlock()
		toLineSync.Lock()
		TimeOutLineStore[resaultBase] = cur
		toLineSync.Unlock()
	}
}

// add value to prometheus
func dataConvert(dataMap map[string]interface{}) {
	var valueArray []string
	metric := dataMap["metric"].(string)
	value := dataMap["value"].(string)
	if metric != "" {
		n, _ := strconv.ParseFloat(value, 64)
		convertKeyArray := mapSort(dataMap)
		for _, v := range convertKeyArray {
			valueArray = append(valueArray, dataMap[v].(string))
		}
		vsSync.Lock()
		ValueStore[metric].(*prometheus.GaugeVec).WithLabelValues(valueArray...).Set(n)
		vsSync.Unlock()
	}
}

// init prometheus struct
func dataInit(dataInitMap map[string]interface{}) {
	var customLabels []string
	var valueArray []string
	metric := dataInitMap["metric"].(string)
	if metric != "" {
		value := dataInitMap["value"].(string)
		n, _ := strconv.ParseFloat(value, 64)
		initKeyArray := mapSort(dataInitMap)
		for _, v := range initKeyArray {
			customLabels = append(customLabels, v)
			valueArray = append(valueArray, dataInitMap[v].(string))
		}
		vsSync.Lock()
		ValueStore[metric] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metric,
			Help: "custom info.",
		}, customLabels)
		metricName := ValueStore[metric]
		prometheus.MustRegister(metricName.(*prometheus.GaugeVec))
		metricName.(*prometheus.GaugeVec).WithLabelValues(valueArray...).Set(n)
		vsSync.Unlock()
	}
}

// Receive custom data.
func customData(res http.ResponseWriter, req *http.Request) {
	body, _ := ioutil.ReadAll(req.Body)
	_, _ = res.Write([]byte(`{"status":"succeed"}`))
	ReqQueue <- string(body)
	_ = req.Body.Close()
}

func timeOutMarkDelete() {
	monitorTimeOut := time.NewTicker(10 * time.Second)
	for {
		<-monitorTimeOut.C
		nowTime := time.Now().Unix()
		toLabelSync.Lock()
		toLineSync.Lock()
		for resaultBase, timeLineStr := range TimeOutLabelStore {
			resaultBytes, _ := base64.StdEncoding.DecodeString(resaultBase)
			timeLine, _ := strconv.ParseInt(timeLineStr, 10, 64)
			lastMarkTime := TimeOutLineStore[resaultBase]
			// delete time out data
			if timeLine < (nowTime - lastMarkTime) {
				var metricInfoTemp map[string]string
				var valueArray []string
				_ = json.Unmarshal(resaultBytes, &metricInfoTemp)
				metric := metricInfoTemp["metric"]
				if metric != "" {
					deleteKeys := make([]string, 0)
					for k := range metricInfoTemp {
						if k != "type" && k != "metric" && k != "timeout" {
							deleteKeys = append(deleteKeys, k)
						}
					}
					for _, v := range deleteKeys {
						valueArray = append(valueArray, metricInfoTemp[v])
					}
					sort.Strings(deleteKeys)
					vsSync.Lock()
					ValueStore[metric].(*prometheus.GaugeVec).DeleteLabelValues(valueArray...)
					vsSync.Unlock()
					delete(TimeOutLabelStore, resaultBase)
					delete(TimeOutLineStore, resaultBase)
				}
			}
		}
		toLabelSync.Unlock()
		toLineSync.Unlock()
	}
}

func kvProcess(bodyStr string, resaultMap map[string]interface{}) {
	_, appOK := resaultMap["app"]
	_, valueOK := resaultMap["value"]
	_, timeoutOK := resaultMap["timeout"]
	_, metricOK := resaultMap["metric"]
	if appOK && valueOK && timeoutOK && metricOK {
		metric := resaultMap["metric"].(string)
		cur := time.Now().Unix()
		lsSync.Lock()
		if _, ok := LabelStore[metric]; ok {
			LabelStore[metric] = cur
			dataConvert(resaultMap)
			timeOutMark(cur, resaultMap)
		} else {
			LabelStore[metric] = cur
			dataInit(resaultMap)
			timeOutMark(cur, resaultMap)
		}
		lsSync.Unlock()
	} else {
		returnData := "Push data is unreasonable. " + bodyStr
		fmt.Println(returnData)
	}
}

func process() {
	numCPU := runtime.NumCPU()
	for i := 1; i <= numCPU; i++ {
		go func() {
			for {
				bodyStr := <-ReqQueue
				printData := "Recive data. " + bodyStr
				fmt.Println(printData)
				resaultMap := make(map[string]interface{})
				_ = json.Unmarshal([]byte(bodyStr), &resaultMap)
				kvProcess(bodyStr, resaultMap)
			}
		}()
	}
}

func Init() {
	LabelStore = make(map[string]int64)
	ValueStore = make(map[string]interface{})
	TimeOutLabelStore = make(map[string]string)
	TimeOutLineStore = make(map[string]int64)
	ReqQueue = make(chan string, 100000)
}

func main() {
	Init()
	go process()
	go timeOutMarkDelete()
	http.HandleFunc("/custom_data/", customData)
	http.Handle("/metrics", promhttp.Handler())
	_ = http.ListenAndServe(":2336", nil)
}
