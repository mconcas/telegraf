// +build !windows

package shim

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShimUSR1SignalingWorks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
		return
	}
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	metricProcessed, exited := runInputPlugin(t, 20*time.Minute, stdinReader, stdoutWriter, nil)

	// signal USR1 to yourself.
	pid := os.Getpid()
	process, err := os.FindProcess(pid)
	require.NoError(t, err)

	go func() {
		// On slow machines this signal can fire before the service comes up.
		// rather than depend on accurate sleep times, we'll just retry sending
		// the signal every so often until it goes through.
		for {
			select {
			case <-ctx.Done():
				return // test is done
			default:
				// test isn't done, keep going.
				process.Signal(syscall.SIGUSR1)
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	<-metricProcessed
	cancel()

	r := bufio.NewReader(stdoutReader)
	out, err := r.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "measurement,tag=tag field=1i 1234000005678\n", out)

	stdinWriter.Close()
	readUntilEmpty(r)

	<-exited
}