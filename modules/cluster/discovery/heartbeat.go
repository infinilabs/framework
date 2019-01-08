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

package discovery

import (
	"context"
	pb "github.com/infinitbyte/framework/core/cluster/pb"
)

type Discovery struct {
}

func (c *Discovery) Join(ctx context.Context, in *pb.JoinRequest) (*pb.JoinResponse, error) {
	out := new(pb.JoinResponse)

	return out, nil
}

func (c *Discovery) Ping(ctx context.Context, in *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	out := new(pb.HealthCheckResponse)
	out.Success = true
	return out, nil
}
