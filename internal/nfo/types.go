package nfo

import "encoding/xml"

// tvshow is the root element of a Jellyfin/Kodi tvshow.nfo document.
// Modeled on the Jellyfin/Kodi NFO schema; xml tags swap the json tags used
// in internal/api/types.go for xml tags (Pitfall 6 — encoding/xml mandatory).
type tvshow struct {
	XMLName  xml.Name  `xml:"tvshow"`
	Title    string    `xml:"title"`
	Plot     string    `xml:"plot,omitempty"`
	Genre    []string  `xml:"genre"`
	Studio   string    `xml:"studio,omitempty"`
	UniqueID uniqueID  `xml:"uniqueid"`
}

// episodedetails is the root element of a per-episode .nfo document.
// The <season> element uses info.EpisodeMetadata.SeasonNumber (Plan 01 fix),
// NOT EpisodeNumber. <plot> uses omitempty so an empty description yields no
// empty <plot></plot> element.
type episodedetails struct {
	XMLName  xml.Name `xml:"episodedetails"`
	Title    string   `xml:"title"`
	ShowTitle string  `xml:"showtitle"`
	Season   int      `xml:"season"`
	Episode  int      `xml:"episode"`
	Plot     string   `xml:"plot,omitempty"`
	UniqueID uniqueID `xml:"uniqueid"`
}

// uniqueID produces <uniqueid type="crunchyroll" default="true">CONTENT_ID</uniqueid>
// via the type,attr + ,chardata pair (D-08). The chardata is the Crunchyroll
// content id used as the stable uniqueid so re-scrapes survive title drift.
type uniqueID struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr,omitempty"`
	Value   string `xml:",chardata"`
}