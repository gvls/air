package runner

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEngine(t *testing.T) {
	_ = os.Unsetenv(airWd)
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	if engine.logger == nil {
		t.Fatal("logger should not be nil")
	}
	if engine.config == nil {
		t.Fatal("config should not be nil")
	}
	if engine.watcher == nil {
		t.Fatal("watcher should not be nil")
	}
}

func TestCheckRunEnv(t *testing.T) {
	_ = os.Unsetenv(airWd)
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	err = engine.checkRunEnv()
	if err == nil {
		t.Fatal("should throw a err")
	}
}

func TestWatching(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	path, err := os.Getwd()
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	path = strings.Replace(path, "_testdata/toml", "", 1)
	err = engine.watching(path + "/_testdata/watching")
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
}

func TestRegexes(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	engine.config.Build.ExcludeRegex = []string{"foo.html$", "bar"}

	result, err := engine.isExcludeRegex("./test/foo.html")
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	if result != true {
		t.Errorf("expected '%t' but got '%t'", true, result)
	}

	result, err = engine.isExcludeRegex("./test/bar/index.html")
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	if result != true {
		t.Errorf("expected '%t' but got '%t'", true, result)
	}

	result, err = engine.isExcludeRegex("./test/unrelated.html")
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	if result {
		t.Errorf("expected '%t' but got '%t'", false, result)
	}
}

func TestRunBin(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}

	err = engine.runBin()
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
}

func GetPort() (int, func()) {
	l, err := net.Listen("tcp", ":0")
	port := l.Addr().(*net.TCPAddr).Port
	if err != nil {
		panic(err)
	}
	return port, func() {
		_ = l.Close()
	}
}

func TestRebuild(t *testing.T) {
	// generate a random port
	port, f := GetPort()
	f()
	t.Logf("port: %d", port)

	tmpDir := initTestEnv(t, port)
	// change dir to tmpDir
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	go func() {
		engine.Run()
		t.Logf("engine stopped")
	}()
	err = waitingPortReady(port, time.Second*5)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	t.Logf("port is ready")

	// start rebuld

	t.Logf("start change main.go")
	// change file of main.go
	// just append a new empty line to main.go
	time.Sleep(time.Second * 2)
	file, err := os.OpenFile("main.go", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	defer file.Close()
	_, err = file.WriteString("\n")
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	err = waitingPortConnectionRefused(port, time.Second*10)
	if err != nil {
		t.Fatalf("timeout: %s.", err)
	}
	t.Logf("connection refused")
	time.Sleep(time.Second * 2)
	err = waitingPortReady(port, time.Second*10)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	t.Logf("port is ready")
	// stop engine
	engine.Stop()
	t.Logf("engine stopped")
}

func waitingPortConnectionRefused(port int, timeout time.Duration) error {
	t := time.NewTimer(timeout)
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	defer t.Stop()
	for {
		select {
		case <-t.C:
			return fmt.Errorf("timeout")
		case <-ticker.C:
			print(".")
			_, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
			if errors.Is(err, syscall.ECONNREFUSED) {
				return nil
			}
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func TestCtrlCWhenREngineIsRunning(t *testing.T) {
	// generate a random port
	port, f := GetPort()
	f()
	t.Logf("port: %d", port)

	tmpDir := initTestEnv(t, port)
	// change dir to tmpDir
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	go func() {
		engine.Run()
		t.Logf("engine stopped")
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		engine.Stop()
		t.Logf("engine stopped")
	}()
	if err := waitingPortReady(port, time.Second*5); err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	sigs <- syscall.SIGINT
	time.Sleep(time.Second * 1)
	err = waitingPortConnectionRefused(port, time.Second*10)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
}

// waitingPortReady waits until the port is ready to be used.
func waitingPortReady(port int, timeout time.Duration) error {
	timeoutChan := time.After(timeout)
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("timeout")
		case <-ticker.C:
			conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

func TestRun(t *testing.T) {
	// generate a random port
	port, f := GetPort()
	f()
	t.Logf("port: %d", port)

	tmpDir := initTestEnv(t, port)
	// change dir to tmpDir
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}

	go func() {
		engine.Run()
	}()
	time.Sleep(time.Second * 2)
	assert.True(t, checkPortHaveBeenUsed(port))
	t.Logf("try to stop")
	engine.Stop()
	time.Sleep(time.Second * 1)
	assert.False(t, checkPortHaveBeenUsed(port))
	t.Logf("stoped")
}

func checkPortHaveBeenUsed(port int) bool {
	conn, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func initTestEnv(t *testing.T, port int) string {
	tempDir := t.TempDir()
	t.Logf("tempDir: %s", tempDir)
	// generate golang code to tempdir
	err := generateGoCode(tempDir, port)
	if err != nil {
		t.Fatalf("Should not be fail: %s.", err)
	}
	return tempDir
}

// generateGoCode generates golang code to tempdir
func generateGoCode(dir string, port int) error {

	code := fmt.Sprintf(`package main

import (
	"log"
	"net/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":%v", nil))
}
`, port)
	file, err := os.Create(dir + "/main.go")
	if err != nil {
		return err
	}
	_, err = file.WriteString(code)

	// generate go mod file
	mod := `module air.sample.com

go 1.17
`
	file, err = os.Create(dir + "/go.mod")
	if err != nil {
		return err
	}
	_, err = file.WriteString(mod)
	if err != nil {
		return err
	}
	return nil
}
