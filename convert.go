package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"golang.org/x/exp/slices"
)

const probeTimeout = 10 * time.Second
const maxFFmpegProbeBytes = 20 * 1024 * 1024

var compatibleVideoCodecs = []string{"h264", "vp9", "hevc"}
var compatibleAudioCodecs = []string{"aac", "opus", "mp3"}

type ffmpegProbeDataStreamsStream struct {
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
}

type ffmpegProbeDataFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
}

type ffmpegProbeData struct {
	Streams []ffmpegProbeDataStreamsStream `json:"streams"`
	Format  ffmpegProbeDataFormat          `json:"format"`
}

type Converter struct {
	VideoCodecs             string
	VideoConvertNeeded      bool
	SingleVideoStreamNeeded bool

	AudioCodecs             string
	AudioConvertNeeded      bool
	SingleAudioStreamNeeded bool

	Duration float64

	UpdateProgressPercentCallback UpdateProgressPercentCallbackFunc
}

func (c *Converter) Probe(rr *ReReadCloser) error {
	defer func() {
		// Restart and replay buffer data used when probing
		rr.Restarted = true
	}()

	fmt.Println("  probing file...")
	i, err := ffmpeg_go.ProbeReaderWithTimeout(io.LimitReader(rr, maxFFmpegProbeBytes), probeTimeout, nil)
	if err != nil {
		return fmt.Errorf("error probing file: %w", err)
	}

	pd := ffmpegProbeData{}
	err = json.Unmarshal([]byte(i), &pd)
	if err != nil {
		return fmt.Errorf("error decoding probe result: %w", err)
	}

	c.Duration, err = strconv.ParseFloat(pd.Format.Duration, 64)
	if err != nil {
		fmt.Println("    error parsing duration:", err)
	}

	gotVideoStream := false
	gotAudioStream := false
	for _, stream := range pd.Streams {
		if stream.CodecType == "video" {
			if c.VideoCodecs != "" {
				c.VideoCodecs += ", "
			}
			c.VideoCodecs += stream.CodecName

			if gotVideoStream {
				fmt.Println("    got additional video stream")
				c.SingleVideoStreamNeeded = true
			} else if !c.VideoConvertNeeded {
				if !slices.Contains(compatibleVideoCodecs, stream.CodecName) {
					fmt.Println("    found incompatible video codec:", stream.CodecName)
					c.VideoConvertNeeded = true
				} else {
					fmt.Println("    found video codec:", stream.CodecName)
				}
				gotVideoStream = true
			}
		} else if stream.CodecType == "audio" {
			if c.AudioCodecs != "" {
				c.AudioCodecs += ", "
			}
			c.AudioCodecs += stream.CodecName

			if gotAudioStream {
				fmt.Println("    got additional audio stream")
				c.SingleAudioStreamNeeded = true
			} else if !c.AudioConvertNeeded {
				if !slices.Contains(compatibleAudioCodecs, stream.CodecName) {
					fmt.Println("    found not compatible audio codec:", stream.CodecName)
					c.AudioConvertNeeded = true
				} else {
					fmt.Println("    found audio codec:", stream.CodecName)
				}
				gotAudioStream = true
			}
		}
	}

	if !gotVideoStream {
		return fmt.Errorf("no video stream found in file")
	}

	return nil
}

func (c *Converter) ffmpegProgressSock() (sockFilename string, sock net.Listener, err error) {
	sockFilename = path.Join(os.TempDir(), fmt.Sprintf("yt-dlp-telegram-bot-%d.sock", rand.Int()))
	sock, err = net.Listen("unix", sockFilename)
	if err != nil {
		fmt.Println("    ffmpeg progress socket create error:", err)
		return
	}

	go func() {
		re := regexp.MustCompile(`out_time_ms=(\d+)\n`)

		fd, err := sock.Accept()
		if err != nil {
			fmt.Println("    ffmpeg progress socket accept error:", err)
			return
		}
		defer fd.Close()

		buf := make([]byte, 64)
		data := ""

		for {
			_, err := fd.Read(buf)
			if err != nil {
				return
			}

			data += string(buf)
			a := re.FindAllStringSubmatch(data, -1)

			if len(a) > 0 && len(a[len(a)-1]) > 0 {
				data = ""
				l, _ := strconv.Atoi(a[len(a)-1][len(a[len(a)-1])-1])
				c.UpdateProgressPercentCallback(int(100 * float64(l) / c.Duration / 1000000))
			}

			if strings.Contains(data, "progress=end") {
				c.UpdateProgressPercentCallback(100)
			}
		}
	}()

	return
}

func (c *Converter) GetActionsNeeded() string {
	var convertNeeded []string
	if c.VideoConvertNeeded || c.SingleVideoStreamNeeded {
		convertNeeded = append(convertNeeded, "video")
	}
	if c.AudioConvertNeeded || c.SingleAudioStreamNeeded {
		convertNeeded = append(convertNeeded, "audio")
	}
	return strings.Join(convertNeeded, ", ")
}

func (c *Converter) ConvertIfNeeded(ctx context.Context, rr *ReReadCloser) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	var cmd *Cmd

	fmt.Print("  converting ", c.GetActionsNeeded(), "...\n")

	args := ffmpeg_go.KwArgs{"format": "mp4", "movflags": "frag_keyframe+empty_moov+faststart"}

	if c.VideoConvertNeeded {
		args = ffmpeg_go.MergeKwArgs([]ffmpeg_go.KwArgs{args, {"c:v": "libx264", "crf": 30, "preset": "veryfast"}})
	} else {
		args = ffmpeg_go.MergeKwArgs([]ffmpeg_go.KwArgs{args, {"c:v": "copy"}})
	}

	if c.AudioConvertNeeded {
		args = ffmpeg_go.MergeKwArgs([]ffmpeg_go.KwArgs{args, {"c:a": "mp3", "q:a": 0}})
	} else {
		args = ffmpeg_go.MergeKwArgs([]ffmpeg_go.KwArgs{args, {"c:a": "copy"}})
	}

	if c.SingleVideoStreamNeeded || c.SingleAudioStreamNeeded {
		args = ffmpeg_go.MergeKwArgs([]ffmpeg_go.KwArgs{args, {"map": "0:v:0,0:a:0"}})
	}

	ff := ffmpeg_go.Input("pipe:0").Output("pipe:1", args)

	var err error
	var progressSock net.Listener
	if c.UpdateProgressPercentCallback != nil {
		if c.Duration > 0 {
			var progressSockFilename string
			progressSockFilename, progressSock, err = c.ffmpegProgressSock()
			if err == nil {
				ff = ff.GlobalArgs("-progress", "unix:"+progressSockFilename)
			}
		} else {
			c.UpdateProgressPercentCallback(-1)
		}
	}

	ffCmd := ff.WithInput(rr).WithOutput(writer).Compile()

	// Creating a new cmd with a timeout context, which will kill the cmd if it takes too long.
	cmd = NewCommand(ctx, ffCmd.Args[0], ffCmd.Args[1:]...)
	cmd.Stdin = ffCmd.Stdin
	cmd.Stdout = ffCmd.Stdout

	// This goroutine handles copying from the input (either rr or cmd.Stdout) to writer.
	go func() {
		err = cmd.Run()
		writer.Close()
		if progressSock != nil {
			progressSock.Close()
		}
	}()

	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("error converting: %w", err)
	}

	return reader, nil
}
