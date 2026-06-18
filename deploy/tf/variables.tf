variable "aws_region" {
  description = "AWS region to deploy the Lambda and schedule into."
  type        = string
  default     = "eu-north-1"
}

variable "name" {
  description = "Name for the Lambda function and its schedule rule. Required."
  type        = string
}

variable "schedule_expression" {
  description = "How often to run the exporter (EventBridge schedule expression)."
  type        = string
  default     = "rate(1 hour)"
}

variable "package_file" {
  description = "Path to the built Lambda zip (a `bootstrap` binary; see the exporter README)."
  type        = string
  default     = "../../function.zip"
}

# --- exporter configuration (becomes the Lambda's environment) ---------------

variable "cx_region" {
  description = "Coralogix region to READ usage from (eu1, eu2, us1, ...)."
  type        = string
}

variable "cx_api_key" {
  description = "Coralogix management API key for reading (needs team-quota-rules:Read)."
  type        = string
  sensitive   = true
}

variable "cx_ingest_key" {
  description = "Coralogix Send-Your-Data key for emitting metrics."
  type        = string
  sensitive   = true
}

variable "cx_team" {
  description = "Value for the `team` label on the emitted metrics."
  type        = string
}

variable "cx_emit_region" {
  description = "Coralogix region to SEND metrics to. Empty means same as cx_region."
  type        = string
  default     = ""
}
