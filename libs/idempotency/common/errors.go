package common

type UniqueConstraintError struct {
	Err error
}

func (e *UniqueConstraintError) Error() string {
	return e.Err.Error()
}
