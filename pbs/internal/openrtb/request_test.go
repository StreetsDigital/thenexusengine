package openrtb

import (
	"encoding/json"
	"testing"
)

func TestBidRequest_JSONRoundTrip(t *testing.T) {
	original := &BidRequest{
		ID: "req-123",
		Imp: []Imp{
			{
				ID:       "imp-1",
				BidFloor: 0.50,
				Banner:   &Banner{W: 300, H: 250},
			},
		},
		Site: &Site{
			ID:     "site-1",
			Domain: "example.com",
			Page:   "https://example.com/page",
		},
		Device: &Device{
			UA: "Mozilla/5.0",
			IP: "192.168.1.1",
		},
		User: &User{
			ID: "user-123",
		},
		AT:   1,
		TMax: 500,
		Cur:  []string{"USD"},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded BidRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %s, want %s", decoded.ID, original.ID)
	}
	if len(decoded.Imp) != 1 {
		t.Fatalf("expected 1 impression, got %d", len(decoded.Imp))
	}
	if decoded.Imp[0].BidFloor != 0.50 {
		t.Errorf("BidFloor mismatch: got %f, want 0.50", decoded.Imp[0].BidFloor)
	}
	if decoded.Site.Domain != "example.com" {
		t.Errorf("Site.Domain mismatch: got %s", decoded.Site.Domain)
	}
	if decoded.TMax != 500 {
		t.Errorf("TMax mismatch: got %d, want 500", decoded.TMax)
	}
}

func TestBidRequest_MinimalValid(t *testing.T) {
	jsonStr := `{"id":"test-1","imp":[{"id":"imp-1","banner":{"w":300,"h":250}}]}`

	var req BidRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.ID != "test-1" {
		t.Errorf("expected id test-1, got %s", req.ID)
	}
	if len(req.Imp) != 1 {
		t.Errorf("expected 1 imp, got %d", len(req.Imp))
	}
}

func TestBidRequest_WithExtension(t *testing.T) {
	jsonStr := `{"id":"test-1","imp":[],"ext":{"prebid":{"debug":true}}}`

	var req BidRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Ext == nil {
		t.Fatal("expected ext to be set")
	}

	var ext map[string]interface{}
	if err := json.Unmarshal(req.Ext, &ext); err != nil {
		t.Fatalf("failed to unmarshal ext: %v", err)
	}

	if ext["prebid"] == nil {
		t.Error("expected prebid in ext")
	}
}

func TestImp_AllMediaTypes(t *testing.T) {
	imp := Imp{
		ID:       "imp-1",
		Banner:   &Banner{W: 300, H: 250},
		Video:    &Video{W: 640, H: 480, Mimes: []string{"video/mp4"}},
		Audio:    &Audio{Mimes: []string{"audio/mp3"}, MinDuration: 15},
		Native:   &Native{Request: `{"ver":"1.2"}`},
		BidFloor: 1.50,
	}

	data, err := json.Marshal(imp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Imp
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Banner == nil {
		t.Error("expected banner")
	}
	if decoded.Video == nil {
		t.Error("expected video")
	}
	if decoded.Audio == nil {
		t.Error("expected audio")
	}
	if decoded.Native == nil {
		t.Error("expected native")
	}
}

func TestImp_SecurePointer(t *testing.T) {
	// Test with secure=1
	secureOne := 1
	imp := Imp{
		ID:     "imp-1",
		Secure: &secureOne,
	}

	data, err := json.Marshal(imp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Imp
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Secure == nil || *decoded.Secure != 1 {
		t.Error("expected secure=1")
	}

	// Test without secure (nil pointer)
	imp2 := Imp{ID: "imp-2"}
	data2, _ := json.Marshal(imp2)

	var decoded2 Imp
	json.Unmarshal(data2, &decoded2)

	if decoded2.Secure != nil {
		t.Error("expected secure to be nil")
	}
}

func TestBanner_Formats(t *testing.T) {
	banner := Banner{
		Format: []Format{
			{W: 300, H: 250},
			{W: 728, H: 90},
			{W: 160, H: 600},
		},
		Pos:   1,
		Mimes: []string{"text/html", "text/javascript"},
	}

	data, err := json.Marshal(banner)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Banner
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Format) != 3 {
		t.Errorf("expected 3 formats, got %d", len(decoded.Format))
	}
	if decoded.Format[0].W != 300 || decoded.Format[0].H != 250 {
		t.Error("first format mismatch")
	}
}

func TestVideo_AllFields(t *testing.T) {
	startDelay := 0
	skip := 1
	video := Video{
		Mimes:          []string{"video/mp4", "video/webm"},
		MinDuration:    5,
		MaxDuration:    30,
		Protocols:      []int{2, 3, 5, 6},
		W:              640,
		H:              480,
		StartDelay:     &startDelay,
		Placement:      1,
		Linearity:      1,
		Skip:           &skip,
		SkipAfter:      5,
		PlaybackMethod: []int{1, 2},
		API:            []int{1, 2},
	}

	data, err := json.Marshal(video)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Video
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Mimes) != 2 {
		t.Errorf("expected 2 mimes, got %d", len(decoded.Mimes))
	}
	if decoded.StartDelay == nil || *decoded.StartDelay != 0 {
		t.Error("expected startdelay=0")
	}
	if decoded.Skip == nil || *decoded.Skip != 1 {
		t.Error("expected skip=1")
	}
}

func TestSite_WithPublisher(t *testing.T) {
	site := Site{
		ID:     "site-123",
		Name:   "Example Site",
		Domain: "example.com",
		Cat:    []string{"IAB1", "IAB2"},
		Page:   "https://example.com/article",
		Ref:    "https://google.com",
		Publisher: &Publisher{
			ID:     "pub-123",
			Name:   "Example Publisher",
			Domain: "publisher.com",
		},
		Content: &Content{
			ID:       "content-1",
			Title:    "Test Article",
			Language: "en",
		},
	}

	data, err := json.Marshal(site)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Site
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Publisher == nil {
		t.Fatal("expected publisher")
	}
	if decoded.Publisher.ID != "pub-123" {
		t.Error("publisher ID mismatch")
	}
	if decoded.Content == nil {
		t.Fatal("expected content")
	}
	if decoded.Content.Title != "Test Article" {
		t.Error("content title mismatch")
	}
}

func TestApp_AllFields(t *testing.T) {
	app := App{
		ID:       "app-123",
		Name:     "Example App",
		Bundle:   "com.example.app",
		Domain:   "example.com",
		StoreURL: "https://play.google.com/store/apps/details?id=com.example.app",
		Cat:      []string{"IAB9"},
		Ver:      "1.0.0",
		Paid:     1,
		Publisher: &Publisher{
			ID:   "pub-456",
			Name: "App Publisher",
		},
	}

	data, err := json.Marshal(app)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded App
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Bundle != "com.example.app" {
		t.Error("bundle mismatch")
	}
	if decoded.Paid != 1 {
		t.Error("expected paid=1")
	}
}

func TestDevice_AllFields(t *testing.T) {
	dnt := 0
	lmt := 1
	device := Device{
		UA:             "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0)",
		IP:             "192.168.1.1",
		IPv6:           "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		Geo:            &Geo{Country: "USA", Region: "CA", City: "Los Angeles"},
		DNT:            &dnt,
		Lmt:            &lmt,
		DeviceType:     4, // Phone
		Make:           "Apple",
		Model:          "iPhone",
		OS:             "iOS",
		OSV:            "14.0",
		W:              375,
		H:              812,
		Language:       "en",
		ConnectionType: 2, // WiFi
		IFA:            "550e8400-e29b-41d4-a716-446655440000",
	}

	data, err := json.Marshal(device)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Device
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.DNT == nil || *decoded.DNT != 0 {
		t.Error("expected dnt=0")
	}
	if decoded.Lmt == nil || *decoded.Lmt != 1 {
		t.Error("expected lmt=1")
	}
	if decoded.Geo == nil || decoded.Geo.Country != "USA" {
		t.Error("geo mismatch")
	}
}

func TestGeo_Coordinates(t *testing.T) {
	geo := Geo{
		Lat:       34.0522,
		Lon:       -118.2437,
		Type:      2, // GPS
		Accuracy:  100,
		Country:   "USA",
		Region:    "CA",
		Metro:     "803",
		City:      "Los Angeles",
		ZIP:       "90001",
		UTCOffset: -480,
	}

	data, err := json.Marshal(geo)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Geo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Lat != 34.0522 {
		t.Errorf("lat mismatch: got %f", decoded.Lat)
	}
	if decoded.Lon != -118.2437 {
		t.Errorf("lon mismatch: got %f", decoded.Lon)
	}
}

func TestUser_WithEIDs(t *testing.T) {
	user := User{
		ID:       "user-123",
		BuyerUID: "buyer-456",
		YOB:      1990,
		Gender:   "M",
		Consent:  "BOEFEAyOEFEAyAHABDENAI4AAAB9vABAASA",
		EIDs: []EID{
			{
				Source: "liveramp.com",
				UIDs: []UID{
					{ID: "XY1234", AType: 3},
				},
			},
			{
				Source: "uidapi.com",
				UIDs: []UID{
					{ID: "uid2-abc", AType: 3},
				},
			},
		},
		Data: []Data{
			{
				ID:   "data-1",
				Name: "Data Provider",
				Segment: []Segment{
					{ID: "seg-1", Value: "high_value"},
				},
			},
		},
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.EIDs) != 2 {
		t.Errorf("expected 2 EIDs, got %d", len(decoded.EIDs))
	}
	if decoded.EIDs[0].Source != "liveramp.com" {
		t.Error("first EID source mismatch")
	}
	if len(decoded.Data) != 1 {
		t.Errorf("expected 1 data segment, got %d", len(decoded.Data))
	}
}

func TestRegs_Privacy(t *testing.T) {
	gdpr := 1
	regs := Regs{
		COPPA:     1,
		GDPR:      &gdpr,
		USPrivacy: "1YNN",
		GPP:       "DBACNYA~CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA",
		GPPSID:    []int{2, 6},
	}

	data, err := json.Marshal(regs)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Regs
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.COPPA != 1 {
		t.Error("expected coppa=1")
	}
	if decoded.GDPR == nil || *decoded.GDPR != 1 {
		t.Error("expected gdpr=1")
	}
	if decoded.USPrivacy != "1YNN" {
		t.Error("us_privacy mismatch")
	}
	if len(decoded.GPPSID) != 2 {
		t.Errorf("expected 2 gpp_sid, got %d", len(decoded.GPPSID))
	}
}

func TestSource_WithSChain(t *testing.T) {
	source := Source{
		FD:     1,
		TID:    "tid-123",
		PChain: "pchain-value",
		SChain: &SupplyChain{
			Complete: 1,
			Ver:      "1.0",
			Nodes: []SupplyChainNode{
				{
					ASI:    "exchange.com",
					SID:    "12345",
					HP:     1,
					Domain: "exchange.com",
				},
				{
					ASI:    "ssp.com",
					SID:    "67890",
					HP:     1,
					Domain: "ssp.com",
				},
			},
		},
	}

	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Source
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.SChain == nil {
		t.Fatal("expected schain")
	}
	if len(decoded.SChain.Nodes) != 2 {
		t.Errorf("expected 2 schain nodes, got %d", len(decoded.SChain.Nodes))
	}
	if decoded.SChain.Nodes[0].ASI != "exchange.com" {
		t.Error("first node ASI mismatch")
	}
}

func TestPMP_WithDeals(t *testing.T) {
	pmp := PMP{
		PrivateAuction: 1,
		Deals: []Deal{
			{
				ID:          "deal-1",
				BidFloor:    5.00,
				BidFloorCur: "USD",
				AT:          1,
				WSeat:       []string{"seat-1", "seat-2"},
			},
			{
				ID:       "deal-2",
				BidFloor: 10.00,
			},
		},
	}

	data, err := json.Marshal(pmp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PMP
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.PrivateAuction != 1 {
		t.Error("expected private_auction=1")
	}
	if len(decoded.Deals) != 2 {
		t.Errorf("expected 2 deals, got %d", len(decoded.Deals))
	}
	if decoded.Deals[0].BidFloor != 5.00 {
		t.Error("first deal bidfloor mismatch")
	}
}

func TestContent_WithProducer(t *testing.T) {
	content := Content{
		ID:         "content-123",
		Episode:    5,
		Title:      "Test Episode",
		Series:     "Test Series",
		Season:     "1",
		Genre:      "Drama",
		URL:        "https://example.com/content",
		Cat:        []string{"IAB1"},
		LiveStream: 0,
		Language:   "en",
		Producer: &Producer{
			ID:     "producer-1",
			Name:   "Test Producer",
			Domain: "producer.com",
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Content
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Producer == nil {
		t.Fatal("expected producer")
	}
	if decoded.Producer.Name != "Test Producer" {
		t.Error("producer name mismatch")
	}
}

func TestAudio_AllFields(t *testing.T) {
	startDelay := 0
	audio := Audio{
		Mimes:       []string{"audio/mp3", "audio/aac"},
		MinDuration: 15,
		MaxDuration: 60,
		Protocols:   []int{9, 10},
		StartDelay:  &startDelay,
		Feed:        1,
		Stitched:    1,
	}

	data, err := json.Marshal(audio)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Audio
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Mimes) != 2 {
		t.Errorf("expected 2 mimes, got %d", len(decoded.Mimes))
	}
	if decoded.StartDelay == nil || *decoded.StartDelay != 0 {
		t.Error("expected startdelay=0")
	}
}

func TestNative_Request(t *testing.T) {
	native := Native{
		Request: `{"ver":"1.2","assets":[{"id":1,"required":1,"title":{"len":50}}]}`,
		Ver:     "1.2",
		API:     []int{3},
	}

	data, err := json.Marshal(native)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Native
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Request == "" {
		t.Error("expected native request")
	}
	if decoded.Ver != "1.2" {
		t.Error("ver mismatch")
	}
}

func TestMetric(t *testing.T) {
	metric := Metric{
		Type:   "viewability",
		Value:  0.85,
		Vendor: "vendor-1",
	}

	data, err := json.Marshal(metric)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Metric
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Type != "viewability" {
		t.Error("type mismatch")
	}
	if decoded.Value != 0.85 {
		t.Errorf("value mismatch: got %f", decoded.Value)
	}
}

// Test parsing real-world OpenRTB request
func TestBidRequest_RealWorld(t *testing.T) {
	jsonStr := `{
		"id": "80ce30c53c16e6ede735f123ef6e32361bfc7b22",
		"at": 1,
		"cur": ["USD"],
		"imp": [{
			"id": "1",
			"bidfloor": 0.03,
			"banner": {
				"h": 250,
				"w": 300,
				"pos": 0
			}
		}],
		"site": {
			"id": "102855",
			"cat": ["IAB3-1"],
			"domain": "www.example.com",
			"page": "http://www.example.com/1234.html",
			"publisher": {
				"id": "8953",
				"name": "example.com"
			}
		},
		"device": {
			"ua": "Mozilla/5.0 (Windows NT 6.1; WOW64)",
			"ip": "64.124.253.1"
		},
		"user": {
			"id": "55816b39711f9b5acf3b90e313ed29e51665623f"
		}
	}`

	var req BidRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to parse real-world request: %v", err)
	}

	if req.ID != "80ce30c53c16e6ede735f123ef6e32361bfc7b22" {
		t.Error("id mismatch")
	}
	if req.AT != 1 {
		t.Error("expected first-price auction (at=1)")
	}
	if len(req.Imp) != 1 {
		t.Error("expected 1 impression")
	}
	if req.Imp[0].BidFloor != 0.03 {
		t.Error("bidfloor mismatch")
	}
	if req.Site.Publisher.Name != "example.com" {
		t.Error("publisher name mismatch")
	}
}

// Benchmark JSON parsing
func BenchmarkBidRequest_Unmarshal(b *testing.B) {
	jsonStr := `{"id":"test-1","imp":[{"id":"imp-1","banner":{"w":300,"h":250}}],"site":{"domain":"example.com"}}`
	data := []byte(jsonStr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var req BidRequest
		json.Unmarshal(data, &req)
	}
}

func BenchmarkBidRequest_Marshal(b *testing.B) {
	req := &BidRequest{
		ID: "test-1",
		Imp: []Imp{
			{ID: "imp-1", Banner: &Banner{W: 300, H: 250}},
		},
		Site: &Site{Domain: "example.com"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(req)
	}
}
