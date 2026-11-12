package app

type SSH struct {
	s *Service
}

func (s *Service) SSH() SSH {
	return SSH{s: s}
}
