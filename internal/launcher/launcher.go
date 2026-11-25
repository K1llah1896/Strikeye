// Package launcher opens a URL with the platform's default handler, used for the
// steam:// links that start CS2 on a specific demo.
package launcher

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Command returns the argv used to hand a URL to the OS. It is separated from
// Open so tests can assert the platform mapping without launching anything.
func Command(goos, url string) ([]string, error) {
	if url == "" {
		return nil, fmt.Errorf("empty url")
	}
	switch goos {
	case "windows":
		// rundll32 avoids the quoting rules of `cmd /c start` and returns at once.
		return []string{"rundll32", "url.dll,FileProtocolHandler", url}, nil
	case "darwin":
		return []string{"open", url}, nil
	default:
		return []string{"xdg-open", url}, nil
	}
}

// Open hands the URL to the OS default handler.
func Open(url string) error {
	argv, err := Command(runtime.GOOS, url)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the child so a long-running handler does not leave a zombie.
	go func() { _ = cmd.Wait() }()
	return nil
}
