package calculator

// PostalZone represents shipping rates for a destination zone
type PostalZone struct {
	HandlingFee    float64            `json:"handlingFee"`
	DiscountBands  map[int]float64    `json:"discountBands"`
	WeightBands    map[string]WeightBand `json:"weightBands"`
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
	BasePricePer100      float64         `json:"basePricePer100"`
	ThresholdAUD         float64         `json:"thresholdAUD"`
	WarningThresholdAUD  float64         `json:"warningThresholdAUD"`
	DiscountBands        map[int]float64 `json:"discountBands"`
}

// Static data - loaded at init
var (
	PostalZones   map[string]PostalZone
	Brands        map[string]Brand
	USATariffs    TariffData
	Zonos         ZonosData
	ExtraCover    ExtraCoverData
	DefaultCOO    = "China"
)

func init() {
	// Initialize postal zones (Australia Post international rates)
	PostalZones = map[string]PostalZone{
		"3-USA & Canada": {
			HandlingFee: 0.02,
			DiscountBands: map[int]float64{
				0: 0, 1: 0.05, 2: 0.15, 3: 0.20, 4: 0.25, 5: 0.30,
			},
			WeightBands: map[string]WeightBand{
				"XSmall": {Label: "XSmall [< 250g]", MaxWeight: 250, BasePrice: 22.30},
				"Small":  {Label: "Small [250 - 500g]", MaxWeight: 500, BasePrice: 29.00},
				"Medium": {Label: "Medium [500 - 1kg]", MaxWeight: 1000, BasePrice: 42.20},
				"Large":  {Label: "Large [1 - 1.5kg]", MaxWeight: 1500, BasePrice: 55.55},
				"XLarge": {Label: "XLarge [1.5kg - 2kg]", MaxWeight: 2000, BasePrice: 68.85},
			},
		},
		"4-UK & Ireland": {
			HandlingFee: 0.02,
			DiscountBands: map[int]float64{
				0: 0, 1: 0.05, 2: 0.15, 3: 0.20, 4: 0.25, 5: 0.30,
			},
			WeightBands: map[string]WeightBand{
				"XSmall": {Label: "XSmall [< 250g]", MaxWeight: 250, BasePrice: 27.50},
				"Small":  {Label: "Small [250 - 500g]", MaxWeight: 500, BasePrice: 34.40},
				"Medium": {Label: "Medium [500 - 1kg]", MaxWeight: 1000, BasePrice: 48.30},
				"Large":  {Label: "Large [1 - 1.5kg]", MaxWeight: 1500, BasePrice: 62.15},
				"XLarge": {Label: "XLarge [1.5kg - 2kg]", MaxWeight: 2000, BasePrice: 76.00},
			},
		},
		"1-New Zealand": {
			HandlingFee: 0.02,
			DiscountBands: map[int]float64{
				0: 0, 1: 0.05, 2: 0.20, 3: 0.25, 4: 0.30, 5: 0.35,
			},
			WeightBands: map[string]WeightBand{
				"XSmall": {Label: "XSmall [< 250g]", MaxWeight: 250, BasePrice: 16.30},
				"Small":  {Label: "Small [250 - 500g]", MaxWeight: 500, BasePrice: 19.65},
				"Medium": {Label: "Medium [500 - 1kg]", MaxWeight: 1000, BasePrice: 26.40},
				"Large":  {Label: "Large [1 - 1.5kg]", MaxWeight: 1500, BasePrice: 33.15},
				"XLarge": {Label: "XLarge [1.5kg - 2kg]", MaxWeight: 2000, BasePrice: 39.90},
			},
		},
	}

	// Initialize brand -> country of origin mappings
	Brands = map[string]Brand{
		"Ada + Lou":           {PrimaryCOO: "Indonesia", SecondaryCOO: []string{}},
		"Aje":                 {PrimaryCOO: "China", SecondaryCOO: []string{"India", "Malaysia"}},
		"Arnhem":              {PrimaryCOO: "Indonesia", SecondaryCOO: []string{}},
		"Auguste":             {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Blue Illusion":       {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Camilla Franks":      {PrimaryCOO: "India", SecondaryCOO: []string{"China"}},
		"Coven & Co":          {PrimaryCOO: "China", SecondaryCOO: []string{"Australia"}},
		"Fillyboo":            {PrimaryCOO: "Indonesia", SecondaryCOO: []string{"India"}},
		"Free People":         {PrimaryCOO: "China", SecondaryCOO: []string{"Vietnam"}},
		"Ghanda":              {PrimaryCOO: "Australia", SecondaryCOO: []string{}},
		"Innika Choo [Bali]":  {PrimaryCOO: "Indonesia", SecondaryCOO: []string{"Vietnam", "Malaysia"}},
		"Innika Choo [China]": {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Innika Choo [India]": {PrimaryCOO: "India", SecondaryCOO: []string{}},
		"Jen's Pirate Booty":  {PrimaryCOO: "Mexico", SecondaryCOO: []string{}},
		"Kivari":              {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Kip & Co":            {PrimaryCOO: "India", SecondaryCOO: []string{}},
		"Lack of Color":       {PrimaryCOO: "China", SecondaryCOO: []string{}, Type: "Hats"},
		"Lele Sadoughi":       {PrimaryCOO: "United States", SecondaryCOO: []string{}, Type: "Headbands"},
		"Love Bonfire":        {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"LoveShackFancy":      {PrimaryCOO: "China", SecondaryCOO: []string{"India"}},
		"Nine Lives Bazaar":   {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Reebok x Maison":     {PrimaryCOO: "Vietnam", SecondaryCOO: []string{}, Type: "Sneakers"},
		"Sabbi":               {PrimaryCOO: "Australia", SecondaryCOO: []string{}},
		"Selkie":              {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Spell":               {PrimaryCOO: "China", SecondaryCOO: []string{}},
		"Tree of Life":        {PrimaryCOO: "India", SecondaryCOO: []string{}},
		"Wildfox":             {PrimaryCOO: "China", SecondaryCOO: []string{"USA"}, Type: "Sunnies"},
	}

	// Initialize US IEEPA tariff rates
	USATariffs = TariffData{
		Rates: map[string]float64{
			"China":         0.20,
			"Malaysia":      0.19,
			"Indonesia":     0.19,
			"Vietnam":       0.20,
			"Japan":         0.15,
			"India":         0.50,
			"Mexico":        0.25,
			"Australia":     0.10,
			"United States": 0.00,
		},
	}

	// Initialize Zonos processing fees
	Zonos = ZonosData{
		ProcessingChargePercent: 0.10,
		FlatFeeAUD:              1.69,
	}

	// Initialize extra cover (insurance) data
	ExtraCover = ExtraCoverData{
		BasePricePer100:     4.00,
		ThresholdAUD:        100,
		WarningThresholdAUD: 250,
		DiscountBands: map[int]float64{
			0: 0, 1: 0.40, 2: 0.40, 3: 0.40, 4: 0.40, 5: 0.40,
		},
	}
}
