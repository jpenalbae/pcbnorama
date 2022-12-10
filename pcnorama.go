package main

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ambelovsky/gosf"
	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
	"go.bug.st/serial"
)

var (
	frames    <-chan []byte
	printer   serial.Port
	debugFlag bool
	stopFlag  bool
)

// Serve the webcam image
// func serveVideoStream(w http.ResponseWriter, req *http.Request) {
// 	mimeWriter := multipart.NewWriter(w)
// 	w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", mimeWriter.Boundary()))
// 	partHeader := make(textproto.MIMEHeader)
// 	partHeader.Add("Content-Type", "image/jpeg")

// 	var frame []byte
// 	for frame = range frames {
// 		if len(frame) == 0 {
// 			log.Print("skipping empty frame")
// 			continue
// 		}

// 		partWriter, err := mimeWriter.CreatePart(partHeader)
// 		if err != nil {
// 			log.Printf("failed to create multi-part writer: %s", err)
// 			return
// 		}

// 		if _, err := partWriter.Write(frame); err != nil {
// 			log.Printf("failed to write image: %s", err)
// 		}

// 	}
// }

func sendToLog(format string, a ...any) {
	message := new(gosf.Message)
	message.Success = true
	message.Text = fmt.Sprintf(format, a...)
	gosf.Broadcast("", "log", message)
}

func printDebug(format string, a ...any) {
	if debugFlag {
		log.Printf(format, a...)
	}
}

func updateWebcam() {
	var frame []byte
	message := new(gosf.Message)

	for frame = range frames {
		if len(frame) == 0 {
			fmt.Print("skipping empty frame")
			continue
		}

		message.Success = true
		message.Text = base64.StdEncoding.EncodeToString(frame)
		gosf.Broadcast("", "webcam", message)
	}
}

func printerInit() {

	buff := make([]byte, 1024)
	printer.SetReadTimeout(2 * time.Second)
	for {
		n, err := printer.Read(buff)
		if err != nil {
			log.Fatal("Could not read serial")
			log.Fatal(err)
		}

		// fmt.Printf("Read bytes %d\n", n)
		// fmt.Printf("%v", string(buff[:n]))
		printDebug("Serial IN %d bytes:\n", n)
		printDebug("%v\n", string(buff[:n]))
		printDebug("------------------------\n")

		if n <= 0 {
			break
		}

	}

	printerSendGcode("G21") // Millimeter Units
	printerSendGcode("G91") // Relative Positioning
}

func printerWaitForOk() {
	buff := make([]byte, 1024)
	printer.SetReadTimeout(10 * time.Second)

	for {
		n, err := printer.Read(buff)
		if err != nil {
			log.Fatal("Could not read serial")
			log.Fatal(err)
		}

		printDebug("Serial IN %d bytes:\n", n)
		printDebug("%v\n", string(buff[:n]))
		printDebug("------------------------\n")

		if strings.Contains(string(buff), "ok") {
			break
		}

		if n <= 0 {
			fmt.Println("Timeout waiting for printer response")
		}

	}
}

func printerSendGcode(gcode string) {
	// printer.ResetInputBuffer()
	n, err := printer.Write([]byte(gcode + "\n\r"))
	if err != nil {
		log.Fatal("Could not write to serial")
		log.Fatal(err)
	}

	printDebug("Serial OUT %d bytes:\n", n)
	printDebug("%s\n", gcode)
	printDebug("------------------------\n")

	printerWaitForOk()
}

func printerMoveAndWait(axis string, mm int) {
	printerSendGcode("G1 " + strings.ToUpper(axis) + strconv.Itoa(mm))
	printerSendGcode("M400") // Wait to finish movement
}

func takeImage(x int, y int) {
	frame := <-frames
	fileName := fmt.Sprintf("results/capture-%d_%d.jpg", y, x)
	file, err := os.Create(fileName)
	if err != nil {
		log.Printf("failed to create file %s: %s", fileName, err)
		return
	}
	if _, err := file.Write(frame); err != nil {
		log.Printf("failed to write file %s: %s", fileName, err)
		return
	}
	if err := file.Close(); err != nil {
		log.Printf("failed to close file %s: %s", fileName, err)
	}
}

func zipResults() {
	file, err := os.Create("static/results.zip")
	if err != nil {
		log.Printf("Could not create results zip file")
		return
	}
	defer file.Close()

	fsys := os.DirFS("results")

	zw := zip.NewWriter(file)
	defer zw.Close()

	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		zf, _ := zw.Create(p)
		f, _ := fsys.Open(p)
		defer f.Close()
		_, _ = io.Copy(zf, f)

		return nil
	})
}

func panoramaStart(width int, heigth int, step int) {
	var x int
	var y int

	stopFlag = false
	printer.ResetInputBuffer()
	printer.ResetOutputBuffer()

	os.RemoveAll("results/")
	os.MkdirAll("results/", 0755)

	for y = 0; y < heigth; y += step {
		for x = 0; x < width; x += step {
			if stopFlag {
				sendToLog("Aborted.")
				return
			}

			if x != 0 {
				printerMoveAndWait("X", step)
			}

			// Wait half sec and take the photo
			time.Sleep(500 * time.Millisecond)
			sendToLog("Taking picture at X: %d Y: %d", x, y)
			takeImage(x, y)
		}
		printerMoveAndWait("X", (x-step)*-1)
		printerMoveAndWait("Y", step*-1)
	}

	// Return home and send done
	printerMoveAndWait("Y", y)

	sendToLog("Compressing results into zip file.")
	zipResults()
	sendToLog("Done.")

}

func panoramaEp(client *gosf.Client, request *gosf.Request) *gosf.Message {
	fmt.Printf("Panorama CMD: %s\n", request.Message.Text)

	defer func() {
		if err := recover(); err != nil {
			log.Println("panic handling panorama cmd:", err)
		}
	}()

	switch request.Message.Text {

	case "start":
		var width int
		var height int
		var step int

		if val, ok := request.Message.Body["width"]; ok {
			width = int(val.(float64))
		}

		if val, ok := request.Message.Body["height"]; ok {
			height = int(val.(float64))
		}

		if val, ok := request.Message.Body["step"]; ok {
			step = int(val.(float64))
		}

		// Check for sanity
		if step > 50 || step < 1 {
			return gosf.NewFailureMessage("Bad steps")
		}

		if width > 500 || width < 5 {
			return gosf.NewFailureMessage("Bad width")
		}

		if height > 500 || height < 5 {
			return gosf.NewFailureMessage("Bad height")
		}

		sendToLog("\nStarting panorama for %dx%dmm with %dmm increments", width, height, step)
		panoramaStart(width, height, step)

	case "stop":
		stopFlag = true

	}

	return gosf.NewSuccessMessage("ok")
}

func printerEp(client *gosf.Client, request *gosf.Request) *gosf.Message {
	fmt.Printf("Printer CMD: %s\n", request.Message.Text)

	defer func() {
		if err := recover(); err != nil {
			log.Println("panic handling printer cmd:", err)
		}
	}()

	switch request.Message.Text {

	case "move":
		var axis string
		var step int

		if val, ok := request.Message.Body["axis"]; ok {
			axis = val.(string)
		}

		if val, ok := request.Message.Body["mm"]; ok {
			step = int(val.(float64))
		}

		if axis != "X" && axis != "Y" && axis != "Z" {
			return gosf.NewFailureMessage("invalid params")
		}

		if step > 100 || step < -100 {
			return gosf.NewFailureMessage("invalid params")
		}

		printerMoveAndWait(axis, step)

	case "homexy":
		printerSendGcode("G28 X Y")
	case "homez":
		printerSendGcode("G28 Z")
	}

	return gosf.NewSuccessMessage("ok")
}

func main() {
	debugFlag = false
	devName := "/dev/video0"
	width := 800
	height := 600
	port := 9001
	wport := 9099
	fps := 30
	dbuffsize := 4
	serialPort := "/dev/ttyUSB0"
	baudRate := 115200

	flag.StringVar(&devName, "d", devName, "device name (path)")
	flag.IntVar(&width, "w", width, "width for the capture resolution")
	flag.IntVar(&height, "h", height, "height for the capture resolution")
	flag.IntVar(&fps, "f", fps, "capture frame rate")
	flag.IntVar(&port, "p", port, "webserver listen port")
	//flag.IntVar(&wport, "P", wport, "websocket listen port")
	flag.StringVar(&serialPort, "s", serialPort, "serial port device")
	flag.IntVar(&baudRate, "b", baudRate, "serial port baud rate")
	flag.BoolVar(&debugFlag, "D", debugFlag, "Print debug messages")
	flag.Parse()

	_, err := os.Stat("./static")
	if err != nil {
		log.Fatal("Missing ./static/ folder with html files")
		log.Fatal(err)
	}

	// open serial
	serialMode := &serial.Mode{
		BaudRate: baudRate,
	}

	tmpserial, err := serial.Open(serialPort, serialMode)
	if err != nil {
		log.Fatalf("Cannot open serial port %s", serialPort)
		log.Fatal(err)
	}
	defer func() {
		tmpserial.Close()
	}()

	printer = tmpserial
	printerInit()
	fmt.Print("Serial ready\n")

	// open webcam
	device, err := device.Open(
		devName,
		device.WithIOType(v4l2.IOTypeMMAP),
		device.WithPixFormat(v4l2.PixFormat{PixelFormat: v4l2.PixelFmtMJPEG, Width: uint32(width), Height: uint32(height)}),
		device.WithFPS(uint32(fps)),
		device.WithBufferSize(uint32(dbuffsize)),
	)

	if err != nil {
		log.Fatalf("failed to open device: %s", err)
		log.Fatal("Camera must support MJPEG format")
	}
	defer device.Close()

	// start stream with cancellable context
	ctx, stop := context.WithCancel(context.TODO())
	if err := device.Start(ctx); err != nil {
		log.Fatalf("failed to start stream: %s", err)
	}
	defer func() {
		stop()
		device.Close()
	}()

	frames = device.GetOutput()
	fmt.Print("Webcam ready\n")

	// Start the websocket
	go func() {
		fmt.Printf("Websocket listening at 0.0.0.0:%d\n", wport)
		gosf.Listen("printer", printerEp)
		gosf.Listen("panorama", panoramaEp)
		gosf.Startup(map[string]interface{}{"port": wport})
	}()
	go updateWebcam()

	// Prepare the web server
	static := http.FileServer(http.Dir("./static"))
	//http.HandleFunc("/stream", serveVideoStream) // returns video feed
	http.Handle("/", static) // serve static content

	portNum := strconv.Itoa(port)
	fmt.Printf("Webserver listening at 0.0.0.0:%d\n", port)
	if err := http.ListenAndServe("0.0.0.0:"+portNum, nil); err != nil {
		log.Fatal(err)
	}

}
