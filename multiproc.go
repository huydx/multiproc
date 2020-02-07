package multiproc

import (
	"bytes"
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
	stderr io.ReadWriter `yaml:"-"`
	stdout io.ReadWriter `yaml:"-"`
	errCh  chan error    `yaml:"-"`
}

func New(cfg *Config) *MultiProc {
	procs := cfg.Procs
	for _, p := range procs {
		p.cancelCh = make(chan struct{}, 1)
	}

	return &MultiProc{
		procs:  procs,
		errCh:  make(chan error, 1),
		stdout: bytes.NewBuffer(make([]byte, 1000)),
		stderr: bytes.NewBuffer(make([]byte, 1000)),
	}
}

func (m *MultiProc) Start() error {
	for _, p := range m.procs {
		go func(pp *Proc) {
			fmt.Printf("try runinng p %v\n", pp)
			cmd := exec.Command(pp.Path, pp.Args...)
			//errPipe, err := cmd.StderrPipe()
			//if err != nil {
			//	m.errCh <- err
			//	return
			//}
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				m.errCh <- err
				return
			}
			err = cmd.Start()
			if err != nil {
				panic(err)
			}
			go func() {
				if _, err := io.Copy(m.stdout, stdoutPipe); err != nil {
					m.errCh <- err
					return
				}
				//if _, err := io.Copy(m.stderr, errPipe); err != nil {
				//	m.errCh <- err
				//	return
				//}
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

			go func() {
				for {
					select {
					case err := <-m.errCh:
						fmt.Println(err)
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
