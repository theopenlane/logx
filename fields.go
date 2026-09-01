package logx

// Canonical log field names shared by the middleware, caller enrichment, and consumers so the
// same concept always lands under the same key
const (
	// FieldRequestID is the log field for the request id
	FieldRequestID = "request_id"
	// FieldOrganizationID is the log field for the organization id
	FieldOrganizationID = "organization_id"
	// FieldSubjectID is the log field for the authenticated subject id
	FieldSubjectID = "subject_id"
	// FieldSubjectEmail is the log field for the authenticated subject email
	FieldSubjectEmail = "subject_email"
	// FieldCapabilities is the log field for the caller capability set
	FieldCapabilities = "capabilities"
	// FieldOperation is the log field for the operation being performed
	FieldOperation = "operation"
	// FieldRemoteIP is the log field for the client origin ip
	FieldRemoteIP = "remote_ip"
	// FieldUserAgent is the log field for the client user agent
	FieldUserAgent = "user_agent"
	// FieldRequestProtocol is the log field for the request protocol
	FieldRequestProtocol = "request_protocol"
	// FieldTrueClientIP is the log field for the True-Client-IP header
	FieldTrueClientIP = "true_client_ip"
	// FieldForwardedFor is the log field for the X-Forwarded-For header
	FieldForwardedFor = "x_forwarded_for"
	// FieldRealIP is the log field for the X-Real-IP header
	FieldRealIP = "x_real_ip"
)
