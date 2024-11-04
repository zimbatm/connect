package evaluation

type TestSession struct {
	SourceIP         string   // needed to know which direction is incoming and which is outgoing
	DataPath         string   // path to raw pcap data
	SavePath         string   // path to save the transport data
	CoOccurrencePath string   // path to save the co-occurrence data
	TimesPath        string   // path to save the times data
	Regions          []Region // fegions for the test session
}

var TestSession1 = TestSession{
	SourceIP:         "145.94.160.91",
	DataPath:         "data/ts1/ts1.pcapng",
	SavePath:         "data/ts1/ts1_transports.pb",
	CoOccurrencePath: "data/ts1/ts1_cooccurrence.pb",
	TimesPath:        "data/ts1/ts1_times.pb",
	Regions: []Region{
		{minT: 11, maxT: 69},
		{minT: 78, maxT: 136},
		{minT: 136, maxT: 185},
		{minT: 194, maxT: 250},
	},
}

var TestSession2 = TestSession{
	SourceIP:         "145.94.190.27",
	DataPath:         "data/ts2/ts2.pcapng",
	SavePath:         "data/ts2/ts2_transports.pb",
	CoOccurrencePath: "data/ts2/ts2_cooccurrence.pb",
	TimesPath:        "data/ts2/ts2_times.pb",
	Regions: []Region{
		{minT: 19, maxT: 50},
		{minT: 58, maxT: 115},
		{minT: 125, maxT: 170},
		{minT: 170, maxT: 230},
		{minT: 238, maxT: 294},
		{minT: 302, maxT: 360},
		{minT: 364, maxT: 410},
		{minT: 415, maxT: 472},
		{minT: 478, maxT: 528},
		{minT: 534, maxT: 570},
		{minT: 579, maxT: 600},
	},
}
