package multiproc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	defaultHealthTimeout = 10 * time.Second
	defaultHttpPort      = 9999
)

type Config struct {
	HttpPort int     `yaml:"http_port"`
	Procs    []*Proc `yaml:"procs"`
}

type Proc struct {
	Path          string        `yaml:"path"`
	HealthPath    string        `yaml:"health"`
	HealthTimeout time.Duration `yaml:"health_time_out"`

	Args     []string         `yaml:"-"`
	cancelCh chan struct{}    `yaml:"-"`
	proc     *os.Process      `yaml:"-"`
	state    *os.ProcessState `yaml:"-"`
}

type MultiProc struct {
	procs    []*Proc
	errCh    chan error `yaml:"-"`
	ioLock   sync.Mutex
	done     chan interface{}
	httpPort int
}

func New(cfg *Config) *MultiProc {
	var httpPort int
	if cfg.HttpPort <= 0 {
		httpPort = defaultHttpPort
	} else {
		httpPort = cfg.HttpPort
	}

	procs := cfg.Procs
	for i, p := range procs {
		if p.HealthTimeout <= 0 {
			procs[i].HealthTimeout = defaultHealthTimeout
		}
		p.cancelCh = make(chan struct{}, 1)
	}

	return &MultiProc{
		procs:    procs,
		httpPort: httpPort,
		errCh:    make(chan error, 1),
		done:     make(chan interface{}, 1),
	}
}

func (m *MultiProc) Start() error {
	go func() {
		http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
			if m.Health() {
				writer.WriteHeader(200)
				_, _ = writer.Write([]byte("ok"))
			} else {
				writer.WriteHeader(500)
				_, _ = writer.Write([]byte("not ok!"))
			}
		})
		http.HandleFunc("/state", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(200)
			_, _ = writer.Write([]byte(m.String()))
		})
		fmt.Printf("start listening at :%d/health\n", m.httpPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", m.httpPort), nil))
	}()
	for i, p := range m.procs {
		var startProc = func(j int, pp *Proc) {
			fmt.Printf("try runinng p %v\n", pp)
			paths := strings.Split(pp.Path, " ")
			if len(paths) == 0 {
				m.errCh <- fmt.Errorf("empty paths")
				return
			}
			cmd := exec.Command(paths[0], paths[1:]...)
			stderrPipe, err := cmd.StderrPipe()
			if err != nil {
				m.errCh <- err
				return
			}
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				m.errCh <- err
				return
			}
			err = cmd.Start()
			if err != nil {
				panic(err)
			}
			m.procs[j].proc = cmd.Process
			m.procs[j].state = cmd.ProcessState
			go func() {
				scanner := bufio.NewScanner(stdoutPipe)
				for scanner.Scan() {
					m.ioLock.Lock()
					_, _ = os.Stdout.Write(scanner.Bytes())
					_, _ = os.Stdout.WriteString("\n")
					m.ioLock.Unlock()
				}
			}()
			go func() {
				scanner := bufio.NewScanner(stderrPipe)
				for scanner.Scan() {
					m.ioLock.Lock()
					_, _ = os.Stderr.Write(scanner.Bytes())
					_, _ = os.Stderr.WriteString("\n")
					m.ioLock.Unlock()
				}
			}()
			if err := cmd.Wait(); err != nil {
				_, _ = os.Stderr.Write([]byte(err.Error()))
			}
			m.procs[i].state = cmd.ProcessState
		}
		go startProc(i, p)
	}

	go func() {
		for {
			select {
			case err := <-m.errCh:
				if err != io.EOF {
					m.ioLock.Lock()
					_, _ = os.Stderr.WriteString(err.Error())
					_, _ = os.Stderr.WriteString("\n")
					m.ioLock.Unlock()
				}
			}
		}
	}()

	for {
		select {
		case <-m.done:
			return nil
		default:
			time.Sleep(time.Second)
		}
	}
}

func (m *MultiProc) Health() bool {
	for _, p := range m.procs {
		r, e := http.NewRequest("GET", p.HealthPath, nil)
		if e != nil {
			_, _ = os.Stderr.WriteString(fmt.Sprintf("process %v died", p.Path))
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), p.HealthTimeout)
		defer cancel()
		r.WithContext(ctx)
		res, err := http.DefaultClient.Do(r)
		if err != nil || res.StatusCode >= 300 {
			_, _ = os.Stderr.WriteString(fmt.Sprintf("process %v died", p.Path))
			return false
		}
	}
	return true
}

func (m *MultiProc) String() string {
	s := strings.Builder{}
	for _, p := range m.procs {
		s.WriteString(p.Path)
		s.WriteString(" state:")
		s.WriteString(p.state.String())
		s.WriteString("\n")
	}
	return s.String()
}

func (m *MultiProc) Stop() error {
	var e error
	var ee *multierror.Error
	for _, p := range m.procs {
		if err := p.proc.Kill(); err != nil {
			m.ioLock.Lock()
			ee = multierror.Append(e, err)
			_, _ = os.Stderr.WriteString(err.Error())
			_, _ = os.Stderr.WriteString("\n")
			m.ioLock.Unlock()
		}
	}
	m.done <- struct{}{}
	if ee == nil {
		return nil
	}
	return ee.ErrorOrNil()
}
