package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got / request\n")
	io.WriteString(w, "This is my website!\n")
}
func getVideo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("v")
	cmd := exec.Command("yt-dlp", "https://www.youtube.com/watch?v="+id, "-o", "-", "--merge-output-format", "mp4")

	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		m := scanner.Bytes()
		w.Write(m)
	}

	cmd.Wait()
}

func main() {
	http.HandleFunc("/", getRoot)
	http.HandleFunc("/watch", getVideo)

	http.ListenAndServe(":3333", nil)
}
