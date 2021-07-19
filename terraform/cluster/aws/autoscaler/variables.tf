variable "cluster" {
  type = object({ certificate_authority : list(object({ data : string })), endpoint : string, id : string, name : string })
}

variable "openid" {
  type = object({ arn : string, url : string })
}

variable "name" {
  type = string
}

variable "tags" {
  type = map(any)
}
