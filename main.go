package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/logging"

	compute "google.golang.org/api/compute/v1"
	appengine "google.golang.org/appengine"
)

type SlackResponse struct {
	Response_type string `json:"response_type"`
	Text          string `json:"text"`
}

func main() {
	// handlerのバインディング
	http.HandleFunc("/server", indexHandler)

	// ポート設定
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// indexHandler responds to requests with our greeting.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	// 環境変数から設定取得する
	project_id := os.Getenv("GCP_PROJECT_NAME")
	zone := os.Getenv("GCP_ZONE")
	instance_name := os.Getenv("GCE_INSTANCE_NAME")
	slack_token := os.Getenv("SLACK_TOKEN")

	// loggerの準備
	ctx := context.Background()
	client, err := logging.NewClient(ctx, project_id)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	logName := "slack-gce-ctrl-log"
	//loggerInfo := client.Logger(logName).StandardLogger(logging.Info)
	loggerDebug := client.Logger(logName).StandardLogger(logging.Debug)
	loggerError := client.Logger(logName).StandardLogger(logging.Error)
	loggerDebug.Println("slack-gce-ctrl api start")

	// check token
	token := r.PostFormValue("token")
	if token != slack_token {
		w.WriteHeader(http.StatusUnauthorized)
		writeSlackMessage(w, "NG")
		loggerError.Printf("wrong slack token: %s", token)
		return
	}

	// ロギング用にPostの情報を取っておく。後でAPIメソッドと合わせてログ出力する
	team_domain := r.PostFormValue("team_domain")
	channel_name := r.PostFormValue("channel_name")
	user_name := r.PostFormValue("user_name")

	str := fmt.Sprintf("team_domain:%s, channel_name:%s, user_name:%s", team_domain, channel_name, user_name)
	loggerDebug.Println(str)

	// check option either status | up | down
	option := r.PostFormValue("text")
	if option == "status" {
		str = fmt.Sprintf("%s requested status.", user_name)
		loggerDebug.Println(str)
		sendExternalIp(w, r, project_id, zone, instance_name)
	} else if option == "up" {
		str = fmt.Sprintf("%s requested up.", user_name)
		loggerDebug.Println(str)
		startInstance(w, r, project_id, zone, instance_name)
	} else if option == "down" {
		str = fmt.Sprintf("%s requested down.", user_name)
		loggerDebug.Println(str)
		stopInstance(w, r, project_id, zone, instance_name)
	} else {
		str = fmt.Sprintf("%s requested invalid option.", user_name)
		loggerDebug.Println(str)
		w.WriteHeader(http.StatusNotAcceptable)
		writeSlackMessage(w, "invalid option")
	}

	loggerDebug.Println("slack-gce-ctrl api end")
}

func writeSlackMessage(w http.ResponseWriter, message string) {
	slack_response := SlackResponse{"in_channel", message}
	json, err := json.Marshal(slack_response)
	if err != nil {
		fmt.Fprint(w, err)
		log.Fatalf("Failed to marshal json: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func sendExternalIp(w http.ResponseWriter, r *http.Request, project string, zone string, instance string) {
	ctx := appengine.NewContext(r)
	s, err := NewComputeService(ctx, w)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	is := compute.NewInstancesService(s)
	insList, err := is.List(project, zone).Do()
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	for _, ins := range insList.Items {
		name := ins.Name
		status := ins.Status
		if name == instance {
			if status == "RUNNING" {
				natIp := ins.NetworkInterfaces[0].AccessConfigs[0].NatIP
				if natIp != "" {
					message := fmt.Sprintf("Minecraft server is running on : %s:25565", natIp)
					writeSlackMessage(w, message)
				}
			} else {
				message := fmt.Sprintf("instance status is %s", status)
				writeSlackMessage(w, message)
			}
		}
	}
}

func startInstance(w http.ResponseWriter, r *http.Request, project string, zone string, instance string) {
	ctx := appengine.NewContext(r)
	s, err := NewComputeService(ctx, w)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	is := compute.NewInstancesService(s)
	insList, err := is.List(project, zone).Do()
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	for _, ins := range insList.Items {
		name := ins.Name
		status := ins.Status
		if name == instance {
			if status == "RUNNING" {
				writeSlackMessage(w, "instance is already RUNNING")
			} else {
				_, err := is.Start(project, zone, instance).Do()
				if err != nil {
					fmt.Fprint(w, err)
					return
				}
				writeSlackMessage(w, "Instance is starting now... wait for minutes..")
			}
		}
	}
}

func stopInstance(w http.ResponseWriter, r *http.Request, project string, zone string, instance string) {
	ctx := appengine.NewContext(r)
	s, err := NewComputeService(ctx, w)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	is := compute.NewInstancesService(s)
	insList, err := is.List(project, zone).Do()
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	for _, ins := range insList.Items {
		name := ins.Name
		status := ins.Status
		if name == instance {
			if status != "RUNNING" {
				message := fmt.Sprintf("instance status is %s", status)
				writeSlackMessage(w, message)
			} else {
				_, err := is.Stop(project, zone, instance).Do()
				if err != nil {
					fmt.Fprint(w, err)
					return
				}
				writeSlackMessage(w, "Instance is stopping now...")
			}
		}
	}
}

func NewComputeService(ctx context.Context, w http.ResponseWriter) (*compute.Service, error) {
	/* client := &http.Client{
		Transport: &oauth2.Transport{
			Source: google.AppEngineTokenSource(ctx, compute.ComputeScope),
			Base:   &urlfetch.Transport{Context: ctx},
		},
	} */
	s, err := compute.NewService(ctx)
	if err != nil {
		fmt.Fprint(w, err)
		return nil, err
	}

	return s, nil
}
