package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var containers = []map[string]any{
	{
		"Id":     "abc123def456ghi789abc123def456ghi789abc123def4",
		"Names":  []string{"/nginx"},
		"Image":  "nginx:1.25",
		"State":  "running",
		"Status": "Up 2 hours",
		"Ports": []map[string]any{
			{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"},
		},
	},
	{
		"Id":     "def456ghi789abc123def456ghi789abc123def456ghi7",
		"Names":  []string{"/redis"},
		"Image":  "redis:7.2",
		"State":  "running",
		"Status": "Up 3 hours",
		"Ports": []map[string]any{
			{"PrivatePort": 6379, "Type": "tcp"},
		},
	},
	{
		"Id":     "ghi789abc123def456ghi789abc123def456ghi789abc1",
		"Names":  []string{"/api"},
		"Image":  "api:v2",
		"State":  "running",
		"Status": "Up 1 hour",
		"Ports":  []map[string]any{},
	},
	{
		"Id":     "jkl012mno345pqr678stu901vwx234yz056789abc123def",
		"Names":  []string{"/worker"},
		"Image":  "worker:latest",
		"State":  "exited",
		"Status": "Exited (0) 5 hours ago",
		"Ports":  []map[string]any{},
	},
}

func getContainer(name string) map[string]any {
	name = strings.TrimPrefix(name, "/")
	for _, c := range containers {
		for _, n := range c["Names"].([]string) {
			if strings.TrimPrefix(n, "/") == name {
				return c
			}
		}
	}
	return nil
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleListContainers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func handleInspectContainer(w http.ResponseWriter, name string) {
	c := getContainer(name)
	if c == nil {
		http.Error(w, "no such container", http.StatusNotFound)
		return
	}

	created, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
	inspect := map[string]any{
		"Id":      c["Id"],
		"Name":    "/" + name,
		"Created": created,
		"State": map[string]any{
			"Status":     c["State"],
			"StartedAt":  "2024-01-15T10:00:01Z",
			"FinishedAt": "0001-01-01T00:00:00Z",
		},
		"Config": map[string]any{
			"Image":  c["Image"],
			"Env":    []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			"Labels": map[string]string{"maintainer": "NGINX Docker Maintainers"},
		},
		"Mounts": []map[string]any{
			{"Type": "bind", "Source": "/data", "Destination": "/usr/share/nginx/html", "Mode": "rw", "RW": true},
		},
		"NetworkSettings": map[string]any{
			"Ports": map[string][]map[string]string{},
		},
		"RestartCount": 1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inspect)
}

func handleStatsContainer(w http.ResponseWriter, r *http.Request) {
	stats := map[string]any{
		"cpu_stats": map[string]any{
			"cpu_usage":        map[string]any{"total_usage": 300000000},
			"system_cpu_usage": 1000000000,
			"online_cpus":      4,
		},
		"precpu_stats": map[string]any{
			"cpu_usage":        map[string]any{"total_usage": 100000000},
			"system_cpu_usage": 500000000,
		},
		"memory_stats": map[string]any{
			"usage": 45 << 20,
			"limit": 512 << 20,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func handleLogsContainer(w http.ResponseWriter, name string) {
	c := getContainer(name)
	if c == nil {
		http.Error(w, "no such container", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	logLines := []string{
		"Starting " + name + " server...",
		"Listening on port 80",
		"INFO: ready to accept connections",
	}
	for _, line := range logLines {
		frame := make([]byte, 8+len(line))
		frame[0] = 1 // stdout
		binary.BigEndian.PutUint32(frame[4:8], uint32(len(line)))
		copy(frame[8:], line)
		w.Write(frame)
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}
}

func handleDiffContainer(w http.ResponseWriter, name string) {
	c := getContainer(name)
	if c == nil {
		http.Error(w, "no such container", http.StatusNotFound)
		return
	}
	changes := []map[string]any{
		{"Path": "/etc/nginx/conf.d/default.conf", "Kind": 0},
		{"Path": "/var/log/nginx/access.log", "Kind": 0},
		{"Path": "/usr/share/nginx/html/index.html", "Kind": 0},
		{"Path": "/tmp/session.lock", "Kind": 1},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(changes)
}

func handleTopContainer(w http.ResponseWriter, r *http.Request) {
	top := map[string]any{
		"Titles": []string{"PID", "USER", "TIME", "COMMAND"},
		"Processes": [][]string{
			{"1", "root", "00:00:01", "nginx -g daemon off;"},
			{"7", "nginx", "00:00:00", "nginx -g daemon off;"},
			{"10", "nginx", "00:00:00", "nginx worker process"},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(top)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		next.ServeHTTP(w, r)
	})
}

func main() {
	socketPath := ""
	if len(os.Args) >= 3 && os.Args[1] == "--socket" {
		socketPath = os.Args[2]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", handlePing)
	mux.HandleFunc("/containers/json", handleListContainers)
	mux.HandleFunc("/containers/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/containers/")
		switch {
		case strings.HasSuffix(path, "/json"):
			name := strings.TrimSuffix(path, "/json")
			handleInspectContainer(w, name)
		case strings.HasSuffix(path, "/stats"):
			handleStatsContainer(w, r)
		case strings.HasSuffix(path, "/logs"):
			name := strings.TrimSuffix(path, "/logs")
			handleLogsContainer(w, name)
		case strings.HasSuffix(path, "/changes"):
			name := strings.TrimSuffix(path, "/changes")
			handleDiffContainer(w, name)
		case strings.HasSuffix(path, "/top"):
			handleTopContainer(w, r)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("PORT=%d\n", tcpListener.Addr().(*net.TCPAddr).Port)

	// Start Unix socket listener when --socket <path> is given
	if socketPath != "" {
		os.Remove(socketPath)
		unixListener, err := net.Listen("unix", socketPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to listen on unix socket: %v\n", err)
			os.Exit(1)
		}
		go http.Serve(unixListener, loggingMiddleware(mux))
	}

	if err := http.Serve(tcpListener, loggingMiddleware(mux)); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
