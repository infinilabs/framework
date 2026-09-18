/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package managed hosts the instance side of the managed-agent subsystem:
// the machinery a centrally managed instance runs to reach back to its
// manager and to serve manager-originated operational calls. It currently
// provides the reverse channel client (reverseclient) and is the home for
// instance-local operational endpoints (log tail, and whatever follows) —
// concerns that are about being managed, not about configuration
// distribution (those stay under infini.sh/framework/modules/configs).
package managed
