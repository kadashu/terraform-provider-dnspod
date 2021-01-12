# terraform-provider-dnspod

Forked from [doitian/terraform-provider-dnspod](https://github.com/doitian/terraform-provider-dnspod), which is no longer maintained.

[Terraform](https://www.terraform.io/) [Provider Plugin](https://www.terraform.io/docs/plugins/provider.html) which manages DNS records in [DNSPod](https://www.dnspod.cn).

## Example

Configure shell environment:

```shell
export DNSPOD_LOGIN_TOKEN="<your-token-id>,<your-token>"
```

Note that it's possible to configure login token via provider argument `login_token`, but it's not recommended.

Config

```tf
provider "dnspod" {
  login_token = "${var.dnspod_login_token}"
}
```

Set an A Record

```tf
resource "dnspod_domain" "example_com" {
    domain = "example.com"
}

resource "dnspod_record" "www_example_com" {
    domain_id = "${dnspod_domain.example_com.id}"
    record_type "A"
    value: "127.0.0.1"
    ttl: 86400
}
```

## Import

To import domain, use the domain ID return from API.

To import record, concatenate its domain id and record id with ":".

## References

- https://docs.dnspod.cn/api/
