package nats

const (
	JobsStreamName = "SHIYAO_JOBS"
	DLQStreamName  = "SHIYAO_DLQ"

	JobsSubject = "shiyao.job.>"
	DLQSubject  = "shiyao.dlq.>"

	SubjectSandboxCreate  = "shiyao.job.sandbox.create"
	SubjectSandboxDestroy = "shiyao.job.sandbox.destroy"
)
