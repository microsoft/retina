package pprof

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var (
	base = "http://localhost:6060/debug/pprof"
	ts   = time.Now().Format("20060102-150405")
)

func downloadPProfResource(endpoint, filename string, queryParams url.Values) error {
	u, err := url.Parse(base + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}
	if queryParams != nil {
		u.RawQuery = queryParams.Encode()
	}

	// Log download information
	fmt.Printf("Downloading: endpoint=%s, output=%s, params=%s\n", endpoint, filename, queryParams.Encode())

	// Start duration counter
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		elapsed := 0
		for {
			select {
			case <-ctx.Done():
				// Clear the counter line
				fmt.Printf("\r%s\r", "                                        ")
				return
			case <-ticker.C:
				elapsed++
				fmt.Printf("\rElapsed: %ds", elapsed)
			}
		}
	}()

	resp, err := http.Get(u.String())
	if err != nil {
		cancel()
		time.Sleep(10 * time.Millisecond) // Give goroutine time to clear
		return fmt.Errorf("failed to GET %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		time.Sleep(10 * time.Millisecond)
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	out, err := os.Create(filename)
	if err != nil {
		cancel()
		time.Sleep(10 * time.Millisecond)
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	cancel()
	time.Sleep(10 * time.Millisecond) // Give goroutine time to clear

	if err != nil {
		return fmt.Errorf("failed to copy body: %w", err)
	}
	return nil
}

func downloadTrace(seconds int, dir string) string {
	params := url.Values{
		"seconds": []string{fmt.Sprintf("%d", seconds)},
	}
	filename := filepath.Join(dir, fmt.Sprintf("trace-%s.out", ts))
	if err := downloadPProfResource("/trace", filename, params); err != nil {
		fmt.Println("Error downloading trace:", err)
		return ""
	}
	return filename
}

func downloadCPUProfile(seconds int, dir string) string {
	params := url.Values{
		"seconds": []string{fmt.Sprintf("%d", seconds)},
	}
	filename := filepath.Join(dir, fmt.Sprintf("cpu-%s.pprof", ts))
	if err := downloadPProfResource("/profile", filename, params); err != nil {
		fmt.Println("Error downloading CPU profile:", err)
		return ""
	}
	return filename
}

func downloadHeap(dir string) string {
	filename := filepath.Join(dir, fmt.Sprintf("heap-%s.pprof", ts))
	if err := downloadPProfResource("/heap", filename, nil); err != nil {
		fmt.Println("Error downloading heap profile:", err)
		return ""
	}
	return filename
}

func downloadGoroutine(dir string) string {
	params := url.Values{
		"debug": []string{"2"},
	}
	filename := filepath.Join(dir, fmt.Sprintf("goroutine-%s.pprof", ts))
	if err := downloadPProfResource("/goroutine", filename, params); err != nil {
		fmt.Println("Error downloading goroutine profile:", err)
		return ""
	}
	return filename
}

func downloadMutex(dir string) string {
	filename := filepath.Join(dir, fmt.Sprintf("mutex-%s.pprof", ts))
	if err := downloadPProfResource("/mutex", filename, nil); err != nil {
		fmt.Println("Error downloading mutex profile:", err)
		return ""
	}
	return filename
}

func DownloadAll(seconds int) error {
	if seconds == 0 {
		fmt.Printf("Seconds set to 0, defaulting to 30\n")
		seconds = 30
	}

	tmpDir, err := os.MkdirTemp("", "pprof-"+ts)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	files := []string{}

	if f := downloadTrace(seconds, tmpDir); f != "" {
		files = append(files, f)
	}

	if f := downloadCPUProfile(seconds, tmpDir); f != "" {
		files = append(files, f)
	}

	if f := downloadHeap(tmpDir); f != "" {
		files = append(files, f)
	}

	if f := downloadGoroutine(tmpDir); f != "" {
		files = append(files, f)
	}

	if f := downloadMutex(tmpDir); f != "" {
		files = append(files, f)
	}

	tarballName := fmt.Sprintf("pprof-profiles-%s.tar.gz", ts)
	if err := createTarball(tarballName, files); err != nil {
		return fmt.Errorf("failed to create tarball: %w", err)
	}

	// Get absolute path to the tarball
	absPath, err := filepath.Abs(tarballName)
	if err != nil {
		absPath = tarballName // fallback to relative path if we can't get absolute
	}

	fmt.Printf("Successfully created %s with %d profiles\n", absPath, len(files))
	return nil
}

func createTarball(tarballPath string, files []string) error {
	tarballFile, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball file: %w", err)
	}
	defer tarballFile.Close()

	gzipWriter := gzip.NewWriter(tarballFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, file := range files {
		if err := addFileToTarball(tarWriter, file); err != nil {
			return fmt.Errorf("failed to add %s to tarball: %w", file, err)
		}
	}

	return nil
}

func addFileToTarball(tarWriter *tar.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use only the base name in the archive
	header.Name = filepath.Base(filename)

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}
