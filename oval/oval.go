/* Vuls - Vulnerability Scanner
Copyright (C) 2016  Future Corporation , Japan.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package oval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cnf "github.com/future-architect/vuls/config"
	"github.com/future-architect/vuls/models"
	"github.com/future-architect/vuls/util"
	"github.com/kotakanbe/goval-dictionary/db"
	"github.com/parnurzeal/gorequest"
)

// Client is the interface of OVAL client.
type Client interface {
	CheckHTTPHealth() error
	FillWithOval(db.DB, *models.ScanResult) (int, error)

	// CheckIfOvalFetched checks if oval entries are in DB by family, release.
	CheckIfOvalFetched(db.DB, string, string) (bool, error)
	CheckIfOvalFresh(db.DB, string, string) (bool, error)
	IsFetchViaHTTP() bool
}

// Base is a base struct
type Base struct {
	family string
}

// CheckHTTPHealth do health check
func (b Base) CheckHTTPHealth() error {
	if !b.IsFetchViaHTTP() {
		return nil
	}

	url := fmt.Sprintf("%s/health", cnf.Conf.OvalDict.URL)
	var errs []error
	var resp *http.Response
	resp, _, errs = gorequest.New().Get(url).End()
	//  resp, _, errs = gorequest.New().SetDebug(config.Conf.Debug).Get(url).End()
	//  resp, _, errs = gorequest.New().Proxy(api.httpProxy).Get(url).End()
	if 0 < len(errs) || resp == nil || resp.StatusCode != 200 {
		return fmt.Errorf("Failed to request to OVAL server. url: %s, errs: %v",
			url, errs)
	}
	return nil
}

// CheckIfOvalFetched checks if oval entries are in DB by family, release.
func (b Base) CheckIfOvalFetched(driver db.DB, osFamily, release string) (fetched bool, err error) {
	if !b.IsFetchViaHTTP() {
		count, err := driver.CountDefs(osFamily, release)
		if err != nil {
			return false, fmt.Errorf("Failed to count OVAL defs: %s, %s, %v",
				osFamily, release, err)
		}
		return 0 < count, nil
	}

	url, _ := util.URLPathJoin(cnf.Conf.OvalDict.URL, "count", osFamily, release)
	resp, body, errs := gorequest.New().Get(url).End()
	if 0 < len(errs) || resp == nil || resp.StatusCode != 200 {
		return false, fmt.Errorf("HTTP GET error: %v, url: %s, resp: %v",
			errs, url, resp)
	}
	count := 0
	if err := json.Unmarshal([]byte(body), &count); err != nil {
		return false, fmt.Errorf("Failed to Unmarshall. body: %s, err: %s",
			body, err)
	}
	return 0 < count, nil
}

// CheckIfOvalFresh checks if oval entries are fresh enough
func (b Base) CheckIfOvalFresh(driver db.DB, osFamily, release string) (ok bool, err error) {
	var lastModified time.Time
	if !b.IsFetchViaHTTP() {
		lastModified = driver.GetLastModified(osFamily, release)
	} else {
		url, _ := util.URLPathJoin(cnf.Conf.OvalDict.URL, "lastmodified", osFamily, release)
		resp, body, errs := gorequest.New().Get(url).End()
		if 0 < len(errs) || resp == nil || resp.StatusCode != 200 {
			return false, fmt.Errorf("HTTP GET error: %v, url: %s, resp: %v",
				errs, url, resp)
		}

		if err := json.Unmarshal([]byte(body), &lastModified); err != nil {
			return false, fmt.Errorf("Failed to Unmarshall. body: %s, err: %s",
				body, err)
		}
	}

	since := time.Now()
	since = since.AddDate(0, 0, -3)
	if lastModified.Before(since) {
		util.Log.Warnf("OVAL for %s %s is old, last modified is %s. It's recommended to update OVAL to improve scanning accuracy. How to update OVAL database, see https://github.com/kotakanbe/goval-dictionary#usage",
			osFamily, release, lastModified)
		return false, nil
	}
	util.Log.Infof("OVAL is fresh: %s %s ", osFamily, release)
	return true, nil
}

// IsFetchViaHTTP checks whether fetch via HTTP
func (b Base) IsFetchViaHTTP() bool {
	// Default value of OvalDBType is sqlite3
	return cnf.Conf.OvalDict.URL != "" && cnf.Conf.OvalDict.Type == "sqlite3"
}
