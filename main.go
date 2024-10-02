package main

import (
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
)

const (
	NoiseDownloadURL = "https://soundproofinglife.com/wp-content/uploads/2023/06/ambiance_brook_calm-20028.mp3"
	DefaultVolume    = 50
)

var (
	NoiseDir  = os.ExpandEnv("$HOME/noise")
	PidFile   = fmt.Sprintf("%v/pid.txt", NoiseDir)
	NoiseFile = fmt.Sprintf("%v/amiance_brook_calm-20028.mp3", NoiseDir)
)

//go:embed icon.png
var icon embed.FS

func main() {
	// resolve home directory
	err := os.MkdirAll(NoiseDir, 0755)
	if err != nil {
		fmt.Println("Failed to create base directory: %v", err)
		panic(err)
	}

	status := getProcessStatus()
	if status == "running" {
		stopProcess()

		// remove pid file if it exists
		if _, err := os.Stat(PidFile); err == nil {
			os.Remove(PidFile)
		}
	}

	http.HandleFunc("/", authMiddleware(indexHandler))
	http.HandleFunc("/start", authMiddleware(startHandler))
	http.HandleFunc("/stop", authMiddleware(stopHandler))
	http.HandleFunc("/volume", authMiddleware(updateVolumeHandler))
	http.Handle("/icon.png", http.FileServer(http.FS(icon)))

	// Setting up signal capturing
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// A go routine which will block until it receives a signal
	go func() {
		<-sigs
		stopProcess()
		os.Exit(0)
	}()

	fmt.Println("Server running on port 8888")
	http.ListenAndServe(":8888", nil)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requiresAuth(r) {
			next.ServeHTTP(w, r)
			return
		}

		username := os.Getenv("AUTH_USER")
		password := os.Getenv("AUTH_PASSWORD")
		if username == "" || password == "" {
			fmt.Println("External access is blocked because AUTH_USER and AUTH_PASSWORD are not set.")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		recvUser, recvPass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(recvUser), []byte(username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(recvPass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func requiresAuth(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return true
	}
	userIP := net.ParseIP(ip)
	if userIP == nil {
		return true
	}

	return !userIP.IsPrivate() && userIP.IsGlobalUnicast()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, `<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<script src="https://cdn.tailwindcss.com"></script>
		<link href="https://cdnjs.cloudflare.com/ajax/libs/flowbite/1.8.1/flowbite.min.css" rel="stylesheet" />
		<script src="https://cdnjs.cloudflare.com/ajax/libs/flowbite/1.8.1/flowbite.min.js"></script>
		<title>Noise Machine</title>
		<link rel="apple-touch-icon" href="/icon.png">
		<link rel="icon" type="image/png" href="/icon.png">

	</head>
	<body class="bg-gray-900 text-white text-center py-20">
		<div class="container mx-auto">
			<h1 class="text-4xl font-bold mb-10 center">Noise Machine</h1>
	`)
	status := getProcessStatus()
	statusLabelColor := "red-500"
	if status == "running" {
		statusLabelColor = "green-500"
	}
	fmt.Fprintf(w, `<p class='block w-3/4 mx-auto text-xl mb-10'>
		<button onclick="location.reload();" class="block mx-auto p-2 text-sm">🔄 Refresh</button>
		<span class='inline-block bg-%v text-white px-3 py-1 rounded-full font-bold'>%v</span>
	</p>`, statusLabelColor, status)

	fmt.Fprintf(w, `
		<a href="#" onclick="start()" class="block w-3/4 mx-auto bg-green-500 p-4 rounded-lg my-4 text-xl font-bold hover:bg-green-700 transition">Start</a>
		<a href="#" onclick="stop()" class="block w-3/4 mx-auto bg-red-500 p-4 rounded-lg my-4 text-xl font-bold hover:bg-red-700 transition">Stop</a>
		<script>
			function start() {
				fetch('/start', { method: 'POST' })
				.then(() => location.reload())
				.catch(err => console.error('Fetch error:', err))
			}
			function stop() {
				fetch('/stop', { method: 'POST' })
				.then(() => location.reload())
				.catch(err => console.error('Fetch error:', err))
			}
		</script>
	`)

	volume, err := getVolume()
	if err != nil {
		fmt.Fprintf(w, "<p>Error getting volume: %v</p>", err)
	} else {
		fmt.Fprintf(w, `
		<div class="block w-3/4 mx-auto text-l">
			<label for="volume-range" class="block mb-2 text-sm font-medium text-white">Volume</label>
			<input id="volume-range" type="range" value="%d" class="w-full h-2 rounded-lg appearance-none cursor-pointer bg-gray-700">
			<script>
				document.getElementById('volume-range').addEventListener('input', () => {
					const volume = document.getElementById('volume-range').value;
					fetch('/volume?volume=' + volume, { method: 'POST' })
					.catch(err => console.error('Error updating volume:', err));
				});
			</script>
		</div>`, volume)
	}

	fmt.Fprintln(w, `</div></body></html>`)
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	err := downloadFileIfNotExists(NoiseFile, NoiseDownloadURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to download file: %s", err), http.StatusInternalServerError)
		return
	}

	status := getProcessStatus()
	if status == "running" {
		fmt.Fprintln(w, "Process is already running")
		return
	}

	cmd := exec.Command("play", "-v", "10", NoiseFile, "repeat", "600")
	err = cmd.Start()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start process: %s", err), http.StatusInternalServerError)
		return
	}

	pid := cmd.Process.Pid
	os.WriteFile(PidFile, []byte(strconv.Itoa(pid)), 0644)

	err = setVolume(DefaultVolume)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to set volume: %s", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Process started with PID %v", pid)
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	err := stopProcess()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop process: %s", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Process stopped")
}

func stopProcess() error {
	status := getProcessStatus()
	if status != "running" {
		return nil
	}

	pid, err := getPidFromFile()
	if err != nil {
		panic(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(PidFile) // if the process doesn't exist but the pid file does, remove it
		return err
	}

	err = process.Kill()
	if err != nil {
		return err
	}

	return os.Remove(PidFile)
}

func downloadFileIfNotExists(filepath string, url string) error {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		out, err := os.Create(filepath)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func getPidFromFile() (int, error) {
	data, err := os.ReadFile(PidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func getProcessStatus() string {
	pid, err := getPidFromFile()
	if err != nil {
		return "not running"
	}

	_, err = os.FindProcess(pid)
	if err != nil {
		os.Remove(PidFile)
		return "not running"
	}

	return "running"
}
func updateVolumeHandler(w http.ResponseWriter, r *http.Request) {
	volumeStr := r.URL.Query().Get("volume")
	volume, err := strconv.Atoi(volumeStr)
	if err != nil {
		http.Error(w, "Invalid volume parameter", http.StatusBadRequest)
		return
	}

	err = setVolume(volume)
	if err != nil {
		http.Error(w, "Error setting volume", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Volume set to %v", volume)
}

func getVolume() (int, error) {
	// Step 1: Run the command
	cmd := exec.Command("amixer", "-M", "sget", "PCM")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Step 2: Parse the response using regex
	regex := regexp.MustCompile(`\[(\d+)%\]`)
	matches := regex.FindStringSubmatch(string(output))
	if matches == nil {
		return 0, errors.New("failed to parse volume")
	}

	// Step 3: Return the volume
	volume, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	return volume, nil
}
func setVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return errors.New("volume must be between 0 and 100")
	}
	volumeStr := strconv.Itoa(volume) + "%"
	cmd := exec.Command("amixer", "-q", "-M", "sset", "PCM", volumeStr)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
