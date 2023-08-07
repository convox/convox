# Letsencrypt dns01 for route53

- Get the iam role that is used by the service:

```
$ convox letsencrypt dns route53 role
arn:aws:iam::047979207916:role/convox/v2-rack162-cert-manager
```

- create route53 dns zone access role with the following permissions(use the appropiate value for zone id):

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "route53:GetChange",
            "Resource": [
                "arn:aws:route53:::change/*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "route53:ChangeResourceRecordSets",
                "route53:ListResourceRecordSets"
            ],
            "Resource": [
                "arn:aws:route53:::hostedzone/<zone-id>"
            ]
        }
    ]
}
```

- Add the cert-manager role as trust policy the above dns access role to give permission to assume:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "",
            "Effect": "Allow",
            "Principal": {
                "AWS": [
                    "arn:aws:iam::047979207916:role/convox/v2-rack162-cert-manager"
                ]
            },
            "Action": "sts:AssumeRole"
        }
    ]
}

```

- Now add following dns access role assume permission policy to cert-manager role(for example: `arn:aws:iam::047979207916:role/convox/v2-rack162-cert-manager`):

```
{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "Statement1",
			"Effect": "Allow",
			"Action": [
				"sts:AssumeRole"
			],
			"Resource": [
				"arn:aws:iam::XXXXXXXXXX:role/dns-access"
			]
		}
	]
}
```

- now configure dns solver for letsencrypt and provide dns zone, dns access role and dns zone id

```
$ convox letsencrypt dns route53 add --id 1 --dns-zones convox.site --role arn:aws:iam::XXXXXX:role/dns-access --hosted-zone-id xxxxxxx
```

check the configuration
```
$convox letsencrypt dns route53 list
ID  DNS-ZONES    HOSTED-ZONE-ID  REGION     ROLE
1   convox.site  XXXXXXXXXXXXX   us-east-1  arn:aws:iam::XXXXXXXXXXXXXXX:role/dns-access
```

now letsencrypt will use dns01 challenge to issue certificate for convox.site
