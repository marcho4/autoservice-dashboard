package adapters

type Server struct {
	Port int
}

func NewServer(port int) *Server {
	return &Server{Port: port}
}

func (s *Server) Run() error {
	return nil
}

func (s *Server) RegisterHandler(path string, func ()) error {
	return nil
}