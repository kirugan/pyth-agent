package pyth

type Status string

const (
	StatusTrading Status = "trading" // when you are publishing a price
	StatusUnknown Status = "unknown" // if you are down for maintenance, or if the market has closed
	StatusHalted  Status = "halted"  // should never be used. Its usually if a ticker has expired i.e. a contract
)
