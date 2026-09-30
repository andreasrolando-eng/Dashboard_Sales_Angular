package auth

import "time"

// Test hooks (compiled only into test binaries).

func init() { bcryptCost = 4 } // fast hashes; production uses 12

// SetClock moves both the service's and the rate limiter's notion of "now".
func (s *Service) SetClock(fn func() time.Time) {
	s.Now = fn
	s.limiter.now = fn
}
