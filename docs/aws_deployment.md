# AWS Deployment Architecture

## Target Architecture
- **Ingress**: AWS ALB
- **Compute**: ECS Fargate services (`api`, `worker`)
- **Database**: Amazon RDS PostgreSQL (Multi-AZ)
- **Queue**: Amazon ElastiCache for Redis
- **Container Registry**: Amazon ECR
- **Secrets**: AWS Secrets Manager
- **Observability**: CloudWatch Logs, CloudWatch Metrics, X-Ray

## Network Topology
- VPC across multiple AZs
- Public subnets: ALB only
- Private subnets: ECS tasks, RDS, ElastiCache
- NAT Gateway for controlled egress

## Security
- TLS termination via ACM + ALB
- Least-privilege IAM task roles
- SG isolation between API, worker, DB, and cache
- Secrets injected from Secrets Manager

## Scalability
- API auto scaling by CPU and ALB request rate
- Worker auto scaling by Redis queue depth metric
- RDS read replicas for heavy history queries

## Deployment Flow
1. GitHub Actions runs tests and builds image.
2. Image is pushed to ECR.
3. ECS service is updated with rolling deployment.
4. ALB health checks validate healthy task replacement.
