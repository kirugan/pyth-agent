package pyth

type SubscriptionNotification struct {
	Jsonrpc string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  SubscriptionParams `json:"params"`
}

type SubscriptionParams struct {
	Subscription int `mapstructure:"subscription" json:"subscription"`
}

type FullProduct struct {
	Account  string `mapstructure:"account"`
	AttrDict struct {
		AssetType     string `mapstructure:"asset_type"`
		Symbol        string `mapstructure:"symbol"`
		Country       string `mapstructure:"country"`
		QuoteCurrency string `mapstructure:"quote_currency"`
		Description   string `mapstructure:"description"`
		Tenor         string `mapstructure:"tenor"`
		GenericSymbol string `mapstructure:"generic_symbol"`
	} `mapstructure:"attr_dict"`
	PriceAccounts []struct {
		Account           string `mapstructure:"account"`
		PriceType         string `mapstructure:"price_type"`
		PriceExponent     int    `mapstructure:"price_exponent"`
		Status            string `mapstructure:"status"`
		Price             int64  `mapstructure:"price"`
		Conf              int    `mapstructure:"conf"`
		EmaPrice          int64  `mapstructure:"ema_price"`
		EmaConfidence     int    `mapstructure:"ema_confidence"`
		ValidSlot         int    `mapstructure:"valid_slot"`
		PubSlot           int    `mapstructure:"pub_slot"`
		PrevSlot          int    `mapstructure:"prev_slot"`
		PrevPrice         int64  `mapstructure:"prev_price"`
		PrevConf          int    `mapstructure:"prev_conf"`
		PublisherAccounts []struct {
			Account string `mapstructure:"account"`
			Status  string `mapstructure:"status"`
			Price   int64  `mapstructure:"price"`
			Conf    int    `mapstructure:"conf"`
			Slot    int    `mapstructure:"slot"`
		} `mapstructure:"publisher_accounts"`
	} `mapstructure:"price_accounts"`
}

type Product struct {
	Account  string `json:"account"`
	AttrDict struct {
		Symbol        string `json:"symbol"`
		AssetType     string `json:"asset_type"`
		Country       string `json:"country"`
		Description   string `json:"description"`
		QuoteCurrency string `json:"quote_currency"`
		Tenor         string `json:"tenor"`
		CmsSymbol     string `json:"cms_symbol"`
		CqsSymbol     string `json:"cqs_symbol"`
		NasdaqSymbol  string `json:"nasdaq_symbol"`
	} `json:"attr_dict"`
	Price []struct {
		Account       string `json:"account"`
		PriceExponent int    `json:"price_exponent"`
		PriceType     string `json:"price_type"`
	} `json:"price"`
}
