package multiproc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
)

type Config struct {
	Procs []*Proc `yaml:"procs"`
}

type Proc struct {
	Path     string          `yaml:"path"`
	Args     []string        `yaml:"-"`
	state    os.ProcessState `yaml:"-"`
	cancelCh chan struct{}   `yaml:"-"`
	proc     *os.Process     `yaml:"-"`
}

type MultiProc struct {
	procs  []*Proc
	errCh  chan error `yaml:"-"`
	ioLock sync.Mutex
	done   chan interface{}
}

func New(cfg *Config) *MultiProc {
	procs := cfg.Procs
	for _, p := range procs {
		p.cancelCh = make(chan struct{}, 1)
	}

	return &MultiProc{
		procs: procs,
		errCh: make(chan error, 1),
		done:  make(chan interface{}, 1),
	}
}

func (m *MultiProc) Start() error {
	for i, p := range m.procs {
		go func(j int, pp *Proc) {
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
		}(i, p)
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
