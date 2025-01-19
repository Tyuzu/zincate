package stripe

type TicketSession struct {
	URL      string
	TicketID string
	EventID  string
	Quantity int
}

func CreateTicketSession(ticketId string, eventId string, quantity int) (TicketSession, error) {
	var s TicketSession
	s.URL = "http://localhost:5173/event/" + eventId
	s.TicketID = ticketId
	s.EventID = eventId
	s.Quantity = quantity
	var err error
	return s, err
}

type MerchSession struct {
	URL     string
	MerchID string
	EventID string
	Stock   int
}

func CreateMerchSession(merchId string, eventId string, stock int) (MerchSession, error) {
	var s MerchSession
	s.URL = "http://localhost:5173/event/" + eventId
	s.MerchID = merchId
	s.EventID = eventId
	s.Stock = stock
	var err error
	return s, err
}
