from __future__ import annotations

# Builtin
import json
from typing import TypedDict, TYPE_CHECKING

# External
import boto3
import click

# Type Checking
if TYPE_CHECKING:
    from mypy_boto3_sts import STSClient


@click.group()
def cli():
    pass


class CredentialProcessOutput(TypedDict):
    Version: int
    AccessKeyId: str
    SecretAccessKey: str
    SessionToken: str
    Expiration: str


@cli.command()
@click.option("--role-arn", required=True, help="ARN of the IAM role to assume")
@click.option(
    "--mfa-serial",
    required=True,
    help="ARN of the virtual MFA to use when assuming the role",
)
@click.option("--profile", required=True, help="Source profile name")
@click.option("--role-session-name", default="ToolkitCLI", help="Role session name")
@click.option(
    "--duration-seconds",
    type=int,
    default=3600,
    help="Role assumption duration seconds",
)
def run_credential_process(
    role_arn: str,
    mfa_serial: str,
    profile: str,
    role_session_name: str,
    duration_seconds: int,
) -> None:
    client: STSClient = boto3.session.Session(profile_name=profile).client("sts")
    token_code: str = click.prompt("Please enter MFA code: ", hide_input=True)
    resp = client.assume_role(
        RoleArn=role_arn,
        RoleSessionName=role_session_name,
        DurationSeconds=duration_seconds,
        SerialNumber=mfa_serial,
        TokenCode=token_code,
    )
    creds = resp["Credentials"]
    output: CredentialProcessOutput = {
        "Version": 1,
        "AccessKeyId": creds["AccessKeyId"],
        "SecretAccessKey": creds["SecretAccessKey"],
        "SessionToken": creds["SessionToken"],
        "Expiration": creds["Expiration"].isoformat(),
    }
    click.echo(json.dumps(output, indent=2))
