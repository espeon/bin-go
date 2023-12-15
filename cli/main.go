package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
)

var serverURL = "http://localhost:8080"

type PasteObj struct {
	Poster    string `json:"Poster"`
	Content   string `json:"Content"`
	Extension string `json:"Extension"`
}

var cmdCreate = &cobra.Command{
	Use:   "create [content]",
	Short: "Create a new paste",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		user, uerr := user.Current()
		var poster string
		if uerr != nil {
			poster = "undefined"
		} else {
			poster = user.Username
		}
		var posterFlag string
		flag.StringVar(&posterFlag, "poster", poster, "Specify the poster")
		flag.Parse()

		if posterFlag != "" {
			poster = posterFlag
		}

		// check if args[0] is a file
		// if it is, read the file and instead, upload a file via multipart/form-data
		file, os_open_err := os.Open(args[0])

		// get the extension from the filename
		// if it's not a file, use the default extension
		extension := "txt"
		if os_open_err == nil {
			fileInfo, err := file.Stat()
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			extension = filepath.Ext(fileInfo.Name())
		}
		var resp *http.Response

		content := args[0]

		// check if it's a file
		if os_open_err == nil {
			// set content to the file's contents
			var buffer bytes.Buffer
			_, err := io.Copy(&buffer, file)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			fileBytes := buffer.Bytes()
			// assume file is utf-8 encoded
			// save file to string
			content = string(fileBytes)
		}

		paste := PasteObj{
			Poster:  poster,
			Content: content,
			// default extension
			Extension: extension,
		}

		payload, err := json.Marshal(paste)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		resp, err = http.Post(serverURL+"/create", "application/json", bytes.NewBuffer(payload))
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Println("Error:", resp.Status)
			return
		}
		fmt.Println("Response:", resp.Status)
		fmt.Println("Access your paste:", serverURL+"/get/"+resp.Header.Get("Slug"))
	},
}

var cmdGet = &cobra.Command{
	Use:   "get [slug]",
	Short: "Get a paste",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := http.Get(serverURL + "/get/" + args[0])
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		fmt.Println("Response:", resp.Status)
		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	},
}

var cmdDelete = &cobra.Command{
	Use:   "delete [slug]",
	Short: "Delete a paste",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		req, err := http.NewRequest("DELETE", serverURL+"/delete/"+args[0], nil)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()
		fmt.Println("Response:", resp.Status)
	},
}

func main() {
	var rootCmd = &cobra.Command{Use: "app"}
	rootCmd.AddCommand(cmdCreate, cmdGet, cmdDelete)
	rootCmd.Execute()
}
