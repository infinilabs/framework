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
	pb "github.com/infinitbyte/framework/core/cluster/pb"

	"context"
)

type RaftServer struct {
}

func (c *RaftServer) AppendEntries(ctx context.Context, in *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	out := new(pb.AppendEntriesResponse)
	return out, nil
}

func (c *RaftServer) RequestVote(ctx context.Context, in *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	out := new(pb.RequestVoteResponse)
	return out, nil
}

func (c *RaftServer) InstallSnapshot(ctx context.Context, in *pb.InstallSnapshotRequest) (*pb.InstallSnapshotResponse, error) {
	out := new(pb.InstallSnapshotResponse)
	return out, nil
}
