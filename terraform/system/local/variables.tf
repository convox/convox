variable "docker_hub_username" {
  default = ""
}

variable "docker_hub_password" {
  default = ""
}

variable "image" {
  default = "convox/convox"
}

variable "name" {
  type = string
}

variable "rack_name" {
  default = ""
  type    = string
}

variable "os" {
  default = "ubuntu"
}

variable "release" {
  default = ""
}

variable "settings" {
  default = ""
}

variable "telemetry" {
  type    = bool
  default = false
}

variable "webhook_signing_key" {
  type        = string
  default     = ""
  description = "Optional HMAC-SHA256 key(s) for signing outbound webhook payloads. Hex-encoded; comma-separated for rotation (max 2). When set, emits Convox-Signature header. Empty preserves 3.24.5 behavior (unsigned)."
}
