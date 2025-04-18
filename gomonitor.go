package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"

	"database/sql"

	hook "github.com/robotn/gohook"
	_ "modernc.org/sqlite"
)

var (
    _, b, _, _ = runtime.Caller(0)
    basepath   = filepath.Dir(b)
	storageRootFilePath = basepath + "/data"
	screenshotImagesPath = filepath.Join(storageRootFilePath, "/screenshots/")
	screenshootDBPath = filepath.Join(storageRootFilePath, "/screenshot.db")
)

type Screenshot struct {
	time string
	filePath  string
}

func takeScreenshot() ([]Screenshot) {
	n := sync.OnceValue(screenshot.NumActiveDisplays)()

	if _, err := os.Stat(screenshotImagesPath); os.IsNotExist(err) {
		os.MkdirAll(screenshotImagesPath, 0700) // Create your file
	}

	screenshotData := make([]Screenshot, 0, 2)

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)

		img, err := screenshot.CaptureRect(bounds)

		if err != nil {
			log.Fatal(err)
		}

		currentDate := time.Now().Format(time.RFC3339)

		fileName := fmt.Sprintf("%s_%dx%d_screen_%d.raw", currentDate, bounds.Dx(), bounds.Dy(), i)

		screenshotFilePath := filepath.Join(screenshotImagesPath, fileName)
		
		file, err := os.Create(screenshotFilePath)

		if err != nil {
			fmt.Println(err)
		}

		file.Write(img.Pix) // To reduce CPU usage, just save the raw file // 0.3%
		// jpeg.Encode(file, img, &jpeg.Options{Quality: 75}) // CPU 2.3%

		file.Close()

		screenshotData = append(screenshotData, Screenshot{
			time: currentDate,
			filePath: screenshotFilePath,
		})
	}

	return screenshotData
}

func main() {
	var screenshotCh = make(chan Screenshot)
	var processListCh = make(chan []string)

	activityCh := hook.Start()
	
	defer hook.End()
	
	db, err := sql.Open("sqlite", screenshootDBPath) 

	if err != nil {
		log.Fatal("Databse can't connected")
	}

	defer db.Close()

	db.Exec("CREATE TABLE IF NOT EXISTS screenshot (id INTEGER PRIMARY KEY NOT NULL, time DATE NOT NULL, path TEXT NOT NULL);") 

	if _, err := os.Stat(storageRootFilePath); os.IsNotExist(err) {
		os.MkdirAll(storageRootFilePath, 0700) // Create your file
	}

	go func() {
		for  {
			screenshots := takeScreenshot()

			for _, v := range(screenshots) {
				screenshotCh <- v 
			}

			time.Sleep(time.Second * 30)
		}
	}()

	go func() {
		for {
			names, err := robotgo.FindNames()

			if err == nil {
				fmt.Println("name: ", names)
			}

			processListCh <- names

			time.Sleep(time.Minute * 5)
		}
	}()
 
	lastActiveTime := time.Now()
	var inactiveTime time.Duration;
	stmt, err := db.Prepare("INSERT INTO screenshot (time, path) VALUES(?, ?)")
	
	for { 
		select {
		case screenShot := <- screenshotCh:
			if err != nil {
				log.Fatal("Error in preparation of insert query", err)
			}

			_, err = stmt.Exec(screenShot.time, screenShot.filePath)

			if err != nil {
				log.Fatal("Error in preparation of insert query", err)
			}
		case activity := <-activityCh:
			checkTime := lastActiveTime.Add(time.Second * 15)

			currentActiveTime := activity.When
			lastActiveTime = currentActiveTime

			if currentActiveTime.Compare(checkTime) == 1  {		
				inactiveTime += currentActiveTime.Sub(checkTime)
				fmt.Println(inactiveTime)
			}
		case processList := <- processListCh:
			fmt.Println(processList)
		}	
	}
}
