package main

import (
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	TRACKLIST string        = "https://vip.aersia.net/roster.xml"
	DOWNLOAD  string        = "download"
	JITTER    time.Duration = 100 * time.Millisecond
)

type Track struct {
	Creator  string `xml:"creator"`
	Title    string `xml:"title"`
	Location string `xml:"location"`
}

func strip(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if ('a' <= b && b <= 'z') ||
			('0' <= b && b <= '9') {
			result.WriteByte(b)
		}
	}
	return result.String()
}

func (t *Track) getName() string {
	return strip(strings.ToLower(t.Title)) + ".m4a"
}

func (t *Track) getSaveLocation() string {
	return path.Join(DOWNLOAD, t.getName())
}

func (t *Track) download() error {
	resp, err := http.Get(t.Location)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(t.getSaveLocation())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

type TrackList struct {
	Tracks []Track `xml:"track"`
}

type Playlist struct {
	XMLName   xml.Name  `xml:"playlist"`
	Version   string    `xml:"version,attr"`
	TrackList TrackList `xml:"trackList"`
}

func worker(track Track) {
	log.Printf("Downloading: %s -> %s", track.Title, track.getSaveLocation())
	if err := track.download(); err != nil {
		log.Fatalf("Track: %+v, failed: %+v", track, err)
	}
}

func main() {
	response, err := http.Get(TRACKLIST)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	var playlist Playlist
	if err := xml.Unmarshal(data, &playlist); err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(DOWNLOAD); os.IsNotExist(err) {
		if err := os.Mkdir(DOWNLOAD, 0755); err != nil {
			log.Fatal(err)
		}
	}
	// Skip first 5 and last 1 as they are junk
	tracks := playlist.TrackList.Tracks[5 : len(playlist.TrackList.Tracks)-1]

	var wg sync.WaitGroup
	limiter := time.Tick(JITTER)
	for _, track := range tracks {
		if _, err := os.Stat(track.getSaveLocation()); os.IsNotExist(err) {
			wg.Add(1)
			go func() {
				<-limiter
				defer wg.Done()
				worker(track)
			}()
		} else {
			log.Printf("Skipping: %s->%s\n", track.Title, track.getSaveLocation())
		}
	}
	wg.Wait()
}
