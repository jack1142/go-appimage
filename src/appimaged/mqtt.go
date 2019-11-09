package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/adrg/xdg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	helpers "github.com/probonopd/appimage/internal/helpers"
	"gopkg.in/ini.v1"
)

func connect(clientId string, uri *url.URL) mqtt.Client {
	opts := createClientOptions(clientId, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	return client
}

func createClientOptions(clientId string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientId)
	return opts
}

func SubscribeMQTT(client mqtt.Client, updateinformation string) {
	time.Sleep(time.Second * 60) // We get retained messages immediately when we subscribe;
	// at this point our AppImage may not be integrated yet...
	// Also it's better user experience not to be bombarded with updates immediately at startup.
	// 60 seconds should be plenty of time.
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	if queryEscapedUpdateInformation == "" {
		return
	}
	topic := helpers.MQTTNamespace + "/" + queryEscapedUpdateInformation + "/#"
	fmt.Println("mqtt: Subscribing for", updateinformation)
	fmt.Println("mqtt: Waiting for messages on topic", helpers.MQTTNamespace+"/"+queryEscapedUpdateInformation+"/version")
	client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		// fmt.Printf("* [%s] %s\n", msg.Topic(), string(msg.Payload()))
		// fmt.Println(topic)
		short := strings.Replace(msg.Topic(), helpers.MQTTNamespace+"/", "", -1)
		parts := strings.Split(short, "/")
		fmt.Println("mqtt: received:", parts)
		if len(parts) < 2 {
			return
		}

		if parts[1] == "version" {
			// version := string(msg.Payload())
			// Decode incoming JSON
			var data helpers.PubSubData
			err := json.Unmarshal(msg.Payload(), &data)
			if err != nil {
				helpers.PrintError("mqtt unmarshal", err)
			}
			version := data.Version
			if version == "" {
				return
			}
			queryEscapedUpdateInformation := parts[0]
			fmt.Println("mqtt:", queryEscapedUpdateInformation, "reports version", version)
			unescapedui, _ := url.QueryUnescape(queryEscapedUpdateInformation)
			if unescapedui == thisai.updateinformation {
				log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++")
				log.Println("+ Update available for this AppImage.")
				log.Println("+ Something special should happen here...")
				log.Println("+ To be imlpemented.")
				log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++")
				SimpleNotify("Update available", "An update for the AppImage daemon is available; I could update myself now...", 0)
			}
			results := FindAppImagesWithMatchingUpdateInformation(unescapedui)
			fmt.Println("mqtt:", updateinformation, "reports version", version, "we have matching", results)
			// Find the most recent local file, based on https://stackoverflow.com/a/45579190
			mostRecent := helpers.FindMostRecentFile(results)
			fmt.Println("mqtt:", updateinformation, "reports version", version, "we have matching", mostRecent)

			// TODO: If version the AppImage has embededed is different, if yes launch AppImageUpdate
			// if helpers.IsCommandAvailable("AppImageUpdate") {
			// 	fmt.Println("mqtt: AppImageUpdate", mostRecent)
			// 	cmd := exec.Command("AppImageUpdate", mostRecent)
			// 	log.Printf("Running AppImageUpdate command and waiting for it to finish...")
			// 	err := cmd.Run()
			// 	log.Printf("AppImageUpdate finished with error: %v", err)
			// }
			ai := newAppImage(mostRecent)
			// TODO: Do some checks before, e.g., see whether we already have it,
			// and whether it is really available for download
			SimpleNotify("Update available", ai.niceName+"\ncan be updated to version "+version, 120000)
		}
	})
}

// FindAppImagesWithMatchingUpdateInformation finds registered AppImages
// that have matching upate information embedded
// FIXME: Take care of things like "appimaged wrap" or "firejail" prefixes. We need to do this differently!
func FindAppImagesWithMatchingUpdateInformation(updateinformation string) []string {
	files, err := ioutil.ReadDir(xdg.DataHome + "/applications/")
	helpers.LogError("desktop", err)
	var results []string
	if err != nil {
		return results
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".desktop") && strings.HasPrefix(file.Name(), "appimagekit_") {
			cfg, e := ini.Load(xdg.DataHome + "/applications/" + file.Name())
			helpers.LogError("desktop", e)
			dst := cfg.Section("Desktop Entry").Key(ExecLocationKey).String()
			_, err = os.Stat(dst)
			if os.IsNotExist(err) {
				log.Println(dst, "does not exist, it is mentioned in", xdg.DataHome+"/applications/"+file.Name())
				continue
			}
			ai := newAppImage(dst)
			ui, err := ai.ReadUpdateInformation()
			if err == nil && ui != "" {
				//log.Println("updateinformation:", ui)
				// log.Println("updateinformation:", url.QueryEscape(ui))
				unescapedui, _ := url.QueryUnescape(ui)
				// log.Println("updateinformation:", unescapedui)
				if updateinformation == unescapedui {
					results = append(results, ai.path)
				}
			}

			continue
		}
	}
	return results
}
