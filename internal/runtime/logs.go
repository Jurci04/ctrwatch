package runtime

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// LogLine represents a single line from a container log stream.
type LogLine struct {
	Container string
	Text      string
	Stream    byte // 1=stdout, 2=stderr
}

// LogOptions controls which log lines are streamed.
type LogOptions struct {
	Tail  string // number of past lines to include (e.g. "100")
	Since string // Unix timestamp, show only lines after this time
}

// StreamLogs returns channels that yield log lines and any terminal error.
// The caller must read from lines until it is closed; the error channel
// delivers at most one value.
func (client *Client) StreamLogs(
	ctx context.Context,
	containerID string,
	opts LogOptions,
) (<-chan LogLine, <-chan error) {
	lines := make(chan LogLine)
	errors := make(chan error, 1)

	go func() {
		defer close(lines)
		defer close(errors)

		query := url.Values{}
		query.Set("stdout", "1")
		query.Set("stderr", "1")
		query.Set("follow", "true")
		if opts.Tail != "" {
			query.Set("tail", opts.Tail)
		}
		if opts.Since != "" {
			query.Set("since", opts.Since)
		}

		req, err := http.NewRequestWithContext(ctx,
			"GET",
			fmt.Sprintf("http://localhost/containers/%s/logs?%s",
				containerID,
				query.Encode()),
			nil,
		)
		if err != nil {
			errors <- err
			return
		}

		resp, err := client.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errors <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errors <- fmt.Errorf("stream logs %s: %s", containerID, resp.Status)
			return
		}

		err = scanLogFrames(resp.Body, containerID, lines)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errors <- fmt.Errorf("scan logs for %s: %w", containerID, err)
			return
		}
	}()

	return lines, errors
}

// scanLogFrames reads a multiplexed stream, strips the 8-byte frame
// headers, and sends each payload as a LogLine.
//
// ponytail: strips multiplexed stream headers (8 bytes per frame);
// upgrades to a proper demuxer if performance on high-throughput streams matters.
const maxFrameSize = 16 * 1024 // max log frame is ~16KB

func scanLogFrames(reader io.Reader, container string, logChan chan<- LogLine) error {
	for {
		var header [8]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
		size := int(binary.BigEndian.Uint32(header[4:8]))
		if size > maxFrameSize {
			return fmt.Errorf("log frame too large: %d", size)
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return err
		}
		logChan <- LogLine{
			Container: container,
			Text:      strings.TrimRight(string(buf), "\n\r"),
			Stream:    header[0],
		}
	}
	return nil
}
