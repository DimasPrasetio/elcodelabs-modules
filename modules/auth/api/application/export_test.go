package application

// RunNow makes the service do its after-the-answer work before returning, so tests can look at
// the result.
func RunNow(s *Service) { s.later = func(f func()) { f() } }
