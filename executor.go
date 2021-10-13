package scriptrunner

type Executor interface {
	Execute(args ...string) (stdOut string, stdErr string, err error)
}
