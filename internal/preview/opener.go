package preview

import (
	"errors"
	"os/exec"
	"runtime"
)

type Runner func(name string, args ...string) error

type Opener struct {
	goos   string
	runner Runner
}

func NewOpener(runner Runner) Opener {
	if runner == nil {
		runner = startCommand
	}
	return Opener{
		goos:   runtime.GOOS,
		runner: runner,
	}
}

func NewOpenerForOS(goos string, runner Runner) Opener {
	if runner == nil {
		runner = startCommand
	}
	return Opener{
		goos:   goos,
		runner: runner,
	}
}

func (o Opener) ShowURL(imageURL string) error {
	name, args, err := commandForOS(o.goos, imageURL)
	if err != nil {
		return err
	}
	return o.runner(name, args...)
}

func commandForOS(goos, imageURL string) (string, []string, error) {
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", imageURL}, nil
	case "darwin":
		return "open", []string{imageURL}, nil
	case "linux":
		return "xdg-open", []string{imageURL}, nil
	default:
		return "", nil, errors.New("opening images is not supported on this platform")
	}
}

func startCommand(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}
