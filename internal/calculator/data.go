package calculator

// PostalZone represents shipping rates for a destination zone
type PostalZone struct {
	HandlingFee   float64                   `json:"handlingFee"`
	DiscountBands map[int]float64           `json:"discountBands"`
	WeightBands   map[string]WeightBand     `json:"weightBands"`
}

// WeightBand represents a weight category with pricing
type WeightBand struct {
	Label     string  `json:"label"`
	MaxWeight int     `json:"maxWeight"` // grams
	BasePrice float64 `json:"basePrice"` // AUD
}

// Brand represents a brand with country of origin info
type Brand struct {
	PrimaryCOO   string   `json:"primaryCOO"`
	SecondaryCOO []string `json:"secondaryCOO"`
	Type         string   `json:"type,omitempty"`
}

// TariffData holds US tariff rates by country
type TariffData struct {
	Rates map[string]float64 `json:"rates"`
}

// ZonosData holds Zonos processing fee info
type ZonosData struct {
	ProcessingChargePercent float64 `json:"processingChargePercent"`
	FlatFeeAUD              float64 `json:"flatFeeAUD"`
}

// ExtraCoverData holds insurance pricing info
type ExtraCoverData struct {
	BasePricePer100     float64         `json:"basePricePer100"`
	ThresholdAUD        float64         `json:"thresholdAUD"`
	WarningThresholdAUD float64         `json:"warningThresholdAUD"`
	DiscountBands       map[int]float64 `json:"discountBands"`
}
