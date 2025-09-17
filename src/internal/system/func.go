package system

import "errors"

// Function is a function that can be started
type Function struct {
	JobId             string
	Cancelable        bool
	Fn                func() error
	Indicator         Indicator
	ParseProgressInfo ParseProgressInfo
	ParseJobError     ParseJobError
}

func NewFunction(jobId string, indicator Indicator, fn func() error) *Function {
	if indicator == nil {
		logger.Warningf("function %s indicator is nil", jobId)
	}
	return &Function{
		JobId:     jobId,
		Indicator: indicator,
		Fn:        fn,
	}
}

// Start start the function
func (f *Function) Start() error {
	if f.Fn == nil {
		return errors.New("function is nil")
	}
	go func() {
		err := f.Fn()
		f.atEnd(err)
	}()
	return nil
}

func (f *Function) atEnd(err error) {
	if f.Indicator == nil {
		logger.Warningf("function %s indicator is nil", f.JobId)
		return
	}
	progress := JobProgressInfo{
		JobId:      f.JobId,
		Cancelable: false,
	}
	if err != nil {
		// failed
		progress.Progress = -1.0
		progress.Status = FailedStatus
		progress.Error = &JobError{
			ErrType:   ErrorUnknown,
			ErrDetail: err.Error(),
		}
		f.Indicator(progress)
		return
	}
	// succeed
	progress.Progress = 1.0
	progress.Status = SucceedStatus
	f.Indicator(progress)
}
