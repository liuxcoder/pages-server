package debug_stepper

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var Enabled = strings.HasSuffix(os.Args[0], ".test") || os.Getenv("DEBUG") == "1"

var Logger = func(s string, i ...interface{}) {
	fmt.Printf(s, i...)
}

type Stepper struct {
	Name       string
	Start      time.Time
	LastStep   time.Time
	Completion time.Time
}

func Start(name string) *Stepper {
	if !Enabled {
		return nil
	}
	t := time.Now()
	Logger("%s: started at %s\n", name, t.Format(time.RFC3339))
	return &Stepper{
		Name:     name,
		Start:    t,
		LastStep: t,
	}
}

func (s *Stepper) Debug(text string) {
	if !Enabled {
		return
	}
	t := time.Now()
	Logger("%s: %s (at %s, %s since last step, %s since start)\n", s.Name, text, t.Format(time.RFC3339), t.Sub(s.LastStep).String(), t.Sub(s.Start).String())
}

func (s *Stepper) Step(description string) {
	if !Enabled {
		return
	}
	if s.Completion != (time.Time{}) {
		Logger("%s: already completed all tasks.\n")
		return
	}
	t := time.Now()
	Logger("%s: completed %s at %s (%s)\n", s.Name, description, t.Format(time.RFC3339), t.Sub(s.LastStep).String())
	s.LastStep = t
}

func (s *Stepper) Complete() {
	if !Enabled {
		return
	}
	if s.Completion != (time.Time{}) {
		Logger("%s: already completed all tasks.\n")
		return
	}
	t := time.Now()
	Logger("%s: completed all tasks at %s (%s since last step; total time: %s)\n", s.Name, t.Format(time.RFC3339), t.Sub(s.LastStep).String(), t.Sub(s.Start).String())
	s.Completion = t
}
