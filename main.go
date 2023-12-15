package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

type PasteObj struct {
	Poster    string
	Content   string
	Extension string
	Slug      string
}

type RootReturnObject struct {
	Engine     string `json:"engine"`
	Version    string `json:"version"`
	PasteCount int    `json:"pasteCount"`
}

func hash(s string) int {
	h := crc32.NewIEEE()
	h.Write([]byte(s))
	return int(h.Sum32())
}

var db *sql.DB

func Hello(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fmt.Fprintf(w, "hello, %s!\n", ps.ByName("name"))
}

func createHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var paste PasteObj
	var id string
	contentType := r.Header.Get("Content-Type")

	if contentType == "application/json" {
		println("json")

		err := json.NewDecoder(r.Body).Decode(&paste)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if contentType == "multipart/form-data" {
		println("multipart")
		// parse multipart form
		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		var buffer bytes.Buffer
		_, err = io.Copy(&buffer, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fileBytes := buffer.Bytes()

		ext := header.Filename[strings.LastIndex(header.Filename, ".")+1:]

		// assume file is utf-8 encoded
		// save file to string
		fileStr := string(fileBytes)

		poster := r.FormValue("poster")
		if poster == "" {
			http.Error(w, "Poster is required", http.StatusBadRequest)
			return
		}
		content := fileStr
		extension := ext

		paste = PasteObj{
			Poster:    poster,
			Content:   content,
			Extension: extension,
		}
	}
	println(paste.Poster)

	orig := strconv.Itoa(hash(paste.Content))
	index := 0
	if len(orig) > index+5 {
		id = orig[index : index+5]
	} else {
		// just backup to random string
		// reset index
		index = 0
		orig = strconv.Itoa(rand.Intn(100000000) + 100000)
		id = orig[index : index+5]
	}

	var exists int
	for {
		if db == nil {
			log.Fatal("Database connection not established")
		}
		err := db.QueryRow("SELECT COUNT (*) FROM pastes WHERE slug = ?", fmt.Sprintf("%s.%s", id, paste.Extension)).Scan(&exists)
		log.Println(exists, fmt.Sprintf("%s.%s", id, paste.Extension))
		if err != nil || exists == 0 {
			if err == sql.ErrNoRows || exists == 0 {
				// id does not exist, we can use it
				break
			} else {
				// handle the error
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		// id exists, generate a new one
		index += 1
		log.Println("index/len", index+5, orig)
		// check if we can still get 5 characters from the original string
		if len(orig) > index+5 {
			id = orig[index : index+5]
		} else {
			// just backup to random string
			// reset index
			index = 0
			orig = strconv.Itoa(rand.Intn(100000000) + 100000)
		}
	}

	paste.Slug = fmt.Sprintf("%s.%s", id, paste.Extension)

	// Save the paste to the db
	if _, err := db.Exec("INSERT INTO pastes (poster, content, slug) VALUES (?, ?, ?)", paste.Poster, paste.Content, paste.Slug); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send the Slug back to the client
	w.Header().Set("Slug", paste.Slug)
	fmt.Fprintf(w, "%s", paste.Slug)
}

func getHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("slug")
	// get paste from sqlite db
	var paste PasteObj

	if err := db.QueryRow("SELECT poster, content, slug FROM pastes WHERE slug = ?", id).Scan(&paste.Poster, &paste.Content, &paste.Slug); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send the paste back to the client
	fmt.Fprintf(w, "%s", paste.Content)
}

func deleteHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("slug")

	t := 121

	// check if paste exists
	if err := db.QueryRow("SELECT id FROM pastes WHERE slug = ?", id).Scan(&t); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// delete paste from sqlite db
	// doesn't seem to work? no error, but no rows affected
	// TODO: fix ASAP
	println(id)
	res, err := db.Exec("DELETE FROM pastes WHERE slug = ?", id)
	if err != nil {
		// handle the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	numDeleted, err := res.RowsAffected()
	if err != nil {
		panic(err)
	}
	println(numDeleted)

	// Send a confirmation back to the client
	fmt.Fprintf(w, "Paste deleted")
}
func main() {
	// Create the sqlite db
	var err error
	db, err = sql.Open("sqlite3", "./pastes.db")
	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()

	// Create table if not exists
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS pastes (id INTEGER PRIMARY KEY, poster TEXT, content TEXT, slug TEXT)"); err != nil {
		log.Fatal(err)
	}

	mux := httprouter.New()

	// return static json object
	mux.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var rows int
		// get amount of pastes from sqlite db
		err := db.QueryRow("SELECT count(DISTINCT id) FROM pastes").Scan(&rows)
		if err != nil {
			log.Fatal(err)
		}

		v := RootReturnObject{
			Engine:     "espeon/bin",
			Version:    "0.0.1",
			PasteCount: rows,
		}
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Println(err)
		}
	})
	mux.POST("/create", createHandler)
	mux.GET("/get/:slug", getHandler)
	mux.DELETE("/delete/:slug", deleteHandler)
	println("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
