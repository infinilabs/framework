/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metadata

//
//
//// sent join requests to seed host
//func join(joinAddr, raftAddr string) error {
//
//	log.Debug("start join address, ", joinAddr, ",", raftAddr)
//	raftAddr = util.GetValidAddress(raftAddr)
//
//	b, err := json.Marshal(map[string]string{"addr": raftAddr})
//	if err != nil {
//		log.Error(err)
//		return err
//	}
//
//	joinAddr = util.GetValidAddress(joinAddr)
//
//	//invalid self clustering
//	if joinAddr == raftAddr {
//		return errors.New(fmt.Sprint("can't cluster with self,", joinAddr, " vs ", raftAddr))
//	}
//
//	if global.Env().SystemConfig.TLSEnabled && len(global.Env().SystemConfig.PathConfig.Cert) > 0 {
//		url := fmt.Sprintf("https://%s/_cluster/node/_join", joinAddr)
//
//		log.Info("try to join the cluster, ", url, ", ", string(b))
//
//		tr := &http.Transport{
//			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
//		}
//		client := &http.Client{Transport: tr}
//		resp, err := client.Post(url, "application-type/json", bytes.NewReader(b))
//		if err != nil {
//			log.Debugf("Get error:", err)
//			return err
//		}
//		defer resp.Body.Close()
//		body, err := ioutil.ReadAll(resp.Body)
//		if err != nil {
//			log.Debugf(url, err)
//			return err
//		}
//		log.Debug(string(body))
//		return nil
//	}
//
//	url := fmt.Sprintf("http://%s/_cluster/node/_join", joinAddr)
//
//	log.Debug("try to join the cluster, ", url, ", ", string(b))
//
//	resp, err := http.Post(url, "application-type/json", bytes.NewReader(b))
//	if err != nil {
//		log.Debug(err)
//		return err
//	}
//
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		log.Debugf(url, err)
//		return err
//	}
//	log.Info("connected to peer: ", joinAddr)
//
//	log.Debug(string(body))
//	defer resp.Body.Close()
//	return nil
//}
//

type RaftModule struct {
}

//
//// handle cache function
//func (s *RaftModule) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
//
//	getKey := func() string {
//		parts := strings.Split(r.URL.Path, "/")
//		if len(parts) != 3 {
//			return ""
//		}
//		return parts[2]
//	}
//
//	switch r.Method {
//
//	case "GET":
//		k := getKey()
//		if k == "" {
//			w.WriteHeader(http.StatusBadRequest)
//		}
//		v, err := s.Get(k)
//		if err != nil {
//			log.Error(err)
//			w.WriteHeader(http.StatusInternalServerError)
//			return
//		}
//
//		b, err := json.Marshal(map[string]string{k: v})
//		if err != nil {
//			log.Error(err)
//			w.WriteHeader(http.StatusInternalServerError)
//			return
//		}
//
//		io.WriteString(w, string(b))
//
//	case "POST":
//		// Read the value from the POST body.
//		m := map[string]string{}
//		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
//			log.Error(err)
//			w.WriteHeader(http.StatusBadRequest)
//			return
//		}
//		for k, v := range m {
//			if err := s.Set(k, v); err != nil {
//				log.Error(err)
//				w.WriteHeader(http.StatusInternalServerError)
//				return
//			}
//		}
//
//	case "DELETE":
//		k := getKey()
//		if k == "" {
//			w.WriteHeader(http.StatusBadRequest)
//			return
//		}
//		if err := s.Delete(k); err != nil {
//			log.Error(err)
//			w.WriteHeader(http.StatusInternalServerError)
//			return
//		}
//		s.Delete(k)
//
//	default:
//		w.WriteHeader(http.StatusMethodNotAllowed)
//	}
//}
//

//
//// handle cluster join function
//func (s *RaftModule) handleJoin(w http.ResponseWriter, r *http.Request) {
//	log.Debug("receive join request")
//
//	m := map[string]string{}
//	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	if len(m) != 1 {
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	remoteAddr, ok := m["addr"]
//	if !ok {
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	if err := s.Join(remoteAddr); err != nil {
//		w.Write([]byte(err.Error()))
//		w.WriteHeader(http.StatusInternalServerError)
//		return
//	}
//	w.Write([]byte(global.Env().SystemConfig.NetworkConfig.RaftBinding))
//}
//
//// handle cluster join function
//func (s *RaftModule) handleLeave(w http.ResponseWriter, r *http.Request) {
//	log.Debug("receive leave request")
//
//	m := map[string]string{}
//	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	if len(m) != 1 {
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	remoteAddr, ok := m["addr"]
//	if !ok {
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	if err := s.Remove(remoteAddr); err != nil {
//		w.Write([]byte(err.Error()))
//		w.WriteHeader(http.StatusInternalServerError)
//		return
//	}
//	w.Write([]byte(global.Env().SystemConfig.NetworkConfig.RaftBinding))
//}

//// Set sets the value for the given key.
//func (s *RaftModule) Set(key, value string) error {
//
//	log.Trace("setting ,", key, ",", value)
//
//	log.Error(s.raft)
//	if s.raft.State() != raft.Leader {
//		return fmt.Errorf("not leader")
//	}
//
//	c := &config.Command{
//		Op:    "set",
//		Key:   key,
//		Value: value,
//	}
//	b, err := json.Marshal(c)
//	if err != nil {
//		return err
//	}
//
//	f := s.raft.Apply(b, raftTimeout)
//	return f.Error()
//}
//
