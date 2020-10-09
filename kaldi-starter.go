package main

import (
	"bbb-kaldi-connector/bbb"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"syscall"

	"github.com/gomodule/redigo/redis"
)

func main() {
	host := "134.100.15.197"
	pubSubConnection, pubSub := bbb.SetupRedisPubSub(host)
	defer pubSubConnection.Close()
	kaldiProcessMap := make(map[string]chan bool)
	redisConnection := bbb.NewRedisConnection(host)
	defer redisConnection.Close()

	for {
		switch v := pubSub.Receive().(type) {
		case redis.Message:
			message := bbb.ParseMessage(v)
			if message.Core.Header.Name == "CreateMeetingReqMsg" {
				meetingID := message.Core.Body.Props.MeetingProp.IntID
				log.Println(meetingID)
				// cmd := exec.Command("bash", "/home/3wille/kaldi-model-server/start.sh")
				cmd := exec.Command(
					"/home/3wille/pykaldi_env/bin/python3", "nnet3_model.py", "-m0", "-e", "-c1", "-t", "-asinc_fastest", "-r 48000", "-ykaldi_tuda_de_nnet3_chain2.yaml",
				)
				log.Println(cmd.Args)
				cmd.Env = append(cmd.Env, "LD_PRELOAD=/opt/intel/mkl/lib/intel64/libmkl_def.so:/opt/intel/mkl/lib/intel64/libmkl_avx2.so:/opt/intel/mkl/lib/intel64/libmkl_core.so:/opt/intel/mkl/lib/intel64/libmkl_intel_lp64.so:/opt/intel/mkl/lib/intel64/libmkl_intel_thread.so:/opt/intel/lib/intel64_lin/libiomp5.so")
				cmd.Dir = "/home/3wille/kaldi-model-server"
				stderr, err := cmd.StderrPipe()
				if err != nil {
					log.Fatal(err)
				}
				if err := cmd.Start(); err != nil {
					log.Fatal(err)
				}
				stopChannel := make(chan bool)
				go func(stopChannel chan bool, cmd exec.Cmd, stderr io.ReadCloser) {
					<-stopChannel
					log.Println("Killing meeting")
					log.Println(cmd.Process.Pid)
					err := cmd.Process.Signal(syscall.SIGINT)
					// err := cmd.Process.Kill()
					if err != nil {
						log.Println("failed to send signal: ", err)
					}
					slurp, _ := ioutil.ReadAll(stderr)
					fmt.Printf("%s\n", slurp)
				}(stopChannel, *cmd, stderr)
				log.Println("Started for meeting: ", meetingID)
				kaldiProcessMap[meetingID] = stopChannel
			} else if message.Core.Header.Name == "DestroyMeetingSysCmdMsg" {
				meetingID := message.Core.Body.MeetingID
				stopChannel, ok := kaldiProcessMap[meetingID]
				if !ok {
					log.Println("no kaldi process found for meeting: ", meetingID)
					continue
				}
				stopChannel <- true
			}
		// case redis.Subscription:
		// 	fmt.Printf("%s: %s %d\n", v.Channel, v.Kind, v.Count)
		case error:
			log.Fatal(v)
		}
	}
}