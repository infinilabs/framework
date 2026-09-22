/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

// FeatureCSRF enables the framework's CSRF filter (modules/security/http_filters)
// for a route. Routes opted in get unsafe methods (POST/PUT/DELETE/PATCH)
// validated against the session-bound CSRF token; see http_filters/csrf.go.
const FeatureCSRF = "feature_csrf"
