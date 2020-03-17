provider "nomad" {
  version = "~> 1.4"
}

variable service_name {
  default = "wasp"
}
variable memory {
  default = "32"
}
variable cpu {
  default = "100"
}
variable replica_count {
  default = "1"
}
variable image_repository {
  default = "vxlabs/wasp"
}
variable image_tag {
    default = "latest"
}
variable args {
  default = []
}

resource "nomad_job" "messages" {
  jobspec = templatefile("${path.module}/template.nomad",
    {
      service_name         = var.service_name,
      replica_count        = var.replica_count,
      service_image        = "${var.image_repository}:${var.image_tag}",
      args                 = var.args,
      memory               = var.memory,
      cpu                  = var.cpu,
    },
  )
}
