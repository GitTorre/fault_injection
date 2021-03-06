package core

import (	
	"os"
	"log"
	"fmt"
	"time"
	"net/http"
	"reflect"
	"strings"
	"strconv"
	"runtime"
	"github.com/opentracing/opentracing-go"
	//zipkin "github.com/openzipkin/zipkin-go-opentracing"
)

const (
	hostPort          = "127.0.0.1:10000"
	collectorEndpoint = "http://localhost:10000/collect"
	sameSpan          = true
	traceID128Bit     = true
)


func CheckInjectError(err error, r *http.Request) {
	if err != nil {
		log.Fatalf("%s: Couldn't inject headers (%v)", r.URL.Path, err)
	}
}


func CheckRequestError(err error, ServiceName string, r *http.Request) {
	if err != nil {
		log.Printf("%s: %s call failed (%v)", r.URL.Path, ServiceName, err)
	}
}


func CheckAndStartSpan(err error, SpanName string, spCtx opentracing.SpanContext) opentracing.Span {
	var sp opentracing.Span
	if err == nil {
		sp = opentracing.StartSpan(SpanName, opentracing.ChildOf(spCtx))
	} else {
		sp = opentracing.StartSpan(SpanName)
	}
	return sp
}



func HandlerDecorator(f http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
	       //fmt.Println("inside decorator")
		//get the name of the handler function
		name := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
		name = strings.Split(name, ".")[1]


		//construct the span to check for faults
		spCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap,
		opentracing.HTTPHeadersCarrier(r.Header))
		sp := CheckAndStartSpan(err, "fault_injection", spCtx)
		
		//if there was a baggage item that signals a fault injection, extract it
		faultRequest := sp.BaggageItem("injectfault")
	    	//fmt.Println("service name: " + name + "  Fault request: " + faultRequest)


		if (faultRequest != "" && strings.Contains(faultRequest, name)) {
			//here, example of faultRequest would be "service4_delay:10" or "service1_drop"
			faultType := strings.Split(faultRequest, "_")[1]
			
			if strings.Compare(faultType, "drop") == 0 {
				//if we requested to drop the packet, do nothing and return
				//maybe it should sleep instead of returning immediately?
				return
			} else if strings.Contains(faultType, ":") {
				//fmt.Println("faultType: " + faultType)

				//here we expect faults in the form "type:value"
				//for example: "delay_ms:10" or "errcode:502"
				compoundFaultType := strings.Split(faultType, ":")
				faultType = compoundFaultType[0]
				faultValue := compoundFaultType[1]
				
				//check if there is a value, if not then it is a bad request
				var value int
				if faultValue == "" {
					fmt.Println("bad fault injection request")
					return
				}else {
					if value, err = strconv.Atoi(faultValue); err != nil {
						fmt.Println("bad value for fault type")
						return
					}
				}	


				switch faultType {
					case "delay":
					time.Sleep(time.Millisecond * time.Duration(value))
						f(w, r) 
					case "errcode":
						//might need to check whether error code is valid
						http.Error(w, http.StatusText(value), value)
						return
					default:
						fmt.Fprintf(os.Stderr, "fault type %s is not supported\n", faultType)
						return
				}
				
			} 			
		}else {
			//if current service is not targeted, simply call the handler
			f(w, r) 
		}
		sp = nil //so that the temporary span does not pollute output
    }
}





















