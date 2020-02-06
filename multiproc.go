package multiproc

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type Config struct {
	Procs []*Proc `yaml:"procs"`
}

type Proc struct {
	Path     string          `yaml:"path"`
	Args     []string        `yaml:"-"`
	state    os.ProcessState `yaml:"-"`
	cancelCh chan struct{}   `yaml:"-"`
}

type MultiProc struct {
	procs  []*Proc
	stderr io.ReadWriteCloser `yaml:"-"`
	stdout io.ReadWriteCloser `yaml:"-"`
	errCh  chan error         `yaml:"-"`
}

func New(cfg *Config) *MultiProc {
	procs := cfg.Procs
	for _, p := range procs {
		p.cancelCh = make(chan struct{}, 1)
	}

	return &MultiProc{
		procs: procs,
		errCh: make(chan error, 1),
	}
}

func (m *MultiProc) Start() error {
	for _, p := range m.procs {
		go func(pp *Proc) {
			fmt.Printf("try runinng p %v¥n", pp)
			cmd := exec.Command(pp.Path, pp.Args...)
			err := cmd.Start()
			if err != nil {
			}
			errPipe, err := cmd.StderrPipe()
			if err != nil {
				m.errCh <- err
				return
			}
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				m.errCh <- err
				return
			}

			go func() {
				for {
					if _, err := io.Copy(m.stdout, stdoutPipe); err != nil {
						m.errCh <- err
						return
					}
					if _, err := io.Copy(m.stderr, errPipe); err != nil {
						m.errCh <- err
						return
					}
				}
			}()

			go func() {
				for {
					buff := make([]byte, 0, 1000)
					n, err := m.stdout.Read(buff)
					if err != nil {
						m.errCh <- err
					}
					if n > 0 {
						fmt.Println(buff)
					}
				}
			}()
		}(p)
	}

	for {
		time.Sleep(time.Second)
	}
}

func (m *MultiProc) Stop() error {
	return nil
}
