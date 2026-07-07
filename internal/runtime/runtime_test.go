package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testClient(fn roundTripFunc) *Client {
	return &Client{httpClient: &http.Client{Transport: fn}}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestListContainersUsesAllQueryAndDecodes(t *testing.T) {
	client := testClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.RawQuery != "all=1" {
			t.Fatalf("query = %q, want all=1", req.URL.RawQuery)
		}
		return response(http.StatusOK, `[{"Id":"abc","Names":["/api"],"Image":"img","State":"running"}]`), nil
	})

	containers, err := client.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 || containers[0].Names[0] != "/api" {
		t.Fatalf("containers = %#v", containers)
	}
}

func TestStatsContainerCalculatesCPUAndMemory(t *testing.T) {
	client := testClient(func(req *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{
			"cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":1000,"online_cpus":2},
			"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":500},
			"memory_stats":{"usage":1024,"limit":2048}
		}`), nil
	})

	stats, err := client.StatsContainer(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	if stats.CPUPercent != 80 || stats.MemoryUsage != 1024 || stats.MemoryLimit != 2048 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestInspectContainerDecodesResponse(t *testing.T) {
	client := testClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/containers/api/json" {
			t.Fatalf("path = %q", req.URL.Path)
		}
		return response(http.StatusOK, `{
			"Id":"abc",
			"Name":"/api",
			"State":{"Status":"running"},
			"Config":{"Image":"example/api","Env":["A=B"],"Labels":{"app":"api"}},
			"RestartCount":2
		}`), nil
	})

	info, err := client.InspectContainer(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "/api" || info.Config.Image != "example/api" || info.RestartCount != 2 {
		t.Fatalf("info = %#v", info)
	}
}

func TestScanLogFramesCleansProgressFrames(t *testing.T) {
	var stream bytes.Buffer
	writeFrame(&stream, 1, "old\rnew\n")
	writeFrame(&stream, 2, "err\n")
	ch := make(chan LogLine, 2)

	if err := scanLogFrames(&stream, "api", ch); err != nil {
		t.Fatal(err)
	}
	close(ch)

	got := []LogLine{<-ch, <-ch}
	if got[0].Text != "new" || got[0].Stream != 1 {
		t.Fatalf("first line = %#v", got[0])
	}
	if got[1].Text != "err" || got[1].Stream != 2 {
		t.Fatalf("second line = %#v", got[1])
	}
}

func TestScanLogFramesRejectsHugeFrame(t *testing.T) {
	var stream bytes.Buffer
	var header [8]byte
	binary.BigEndian.PutUint32(header[4:8], maxFrameSize+1)
	stream.Write(header[:])

	if err := scanLogFrames(&stream, "api", make(chan LogLine)); err == nil {
		t.Fatal("expected huge frame error")
	}
}

func writeFrame(w *bytes.Buffer, stream byte, payload string) {
	var header [8]byte
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
	w.Write(header[:])
	w.WriteString(payload)
}
