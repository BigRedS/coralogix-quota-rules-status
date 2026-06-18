# Deploys quota-rules-status-exporter as a scheduled AWS Lambda.
#
# Build the package first (from the repo root):
#   GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
#     go build -trimpath -ldflags "-s -w" -o bootstrap ./cmd/quota-rules-status-exporter
#   zip function.zip bootstrap
#
# Then, from this directory:
#   tofu init
#   tofu apply -var name=quota-rules-status -var cx_region=eu2 \
#       -var cx_team=otel-demo -var cx_api_key=... -var cx_ingest_key=...

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# --- IAM role: just enough for the function to write its own logs -------------

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "exporter" {
  name               = "${var.name}-role"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_iam_role_policy_attachment" "logs" {
  role       = aws_iam_role.exporter.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# --- the function -------------------------------------------------------------

resource "aws_lambda_function" "exporter" {
  function_name    = var.name
  role             = aws_iam_role.exporter.arn
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  architectures    = ["arm64"]
  filename         = var.package_file
  source_code_hash = filebase64sha256(var.package_file)
  timeout          = 30

  environment {
    variables = {
      CX_REGION             = var.cx_region
      CX_API_KEY            = var.cx_api_key
      CX_SEND_YOUR_DATA_KEY = var.cx_ingest_key
      CX_TEAM               = var.cx_team
      CX_EMIT_REGION        = var.cx_emit_region
    }
  }
}

# --- the schedule -------------------------------------------------------------

resource "aws_cloudwatch_event_rule" "schedule" {
  name                = var.name
  schedule_expression = var.schedule_expression
}

resource "aws_cloudwatch_event_target" "schedule" {
  rule = aws_cloudwatch_event_rule.schedule.name
  arn  = aws_lambda_function.exporter.arn
}

resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.exporter.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedule.arn
}

output "function_arn" {
  value = aws_lambda_function.exporter.arn
}
