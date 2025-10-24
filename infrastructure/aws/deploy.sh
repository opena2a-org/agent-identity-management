#!/bin/bash

################################################################################
# AIM - AWS Deployment Script
################################################################################
# Description: Deploy AIM to AWS using ECS Fargate
# Author: AIM Team
# Version: 1.0.0
# License: Apache 2.0
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
AWS_REGION="${AWS_REGION:-us-east-1}"
CLUSTER_NAME="${AIM_CLUSTER:-aim-cluster}"
SERVICE_NAME="${AIM_SERVICE:-aim}"
ECR_REPO_BACKEND="${AIM_ECR_BACKEND:-aim-backend}"
ECR_REPO_FRONTEND="${AIM_ECR_FRONTEND:-aim-frontend}"
RDS_INSTANCE="${AIM_RDS_INSTANCE:-aim-postgres}"
REDIS_CLUSTER="${AIM_REDIS:-aim-redis}"
VPC_NAME="${AIM_VPC:-aim-vpc}"

################################################################################
# Helper Functions
################################################################################

print_header() {
    echo -e "${PURPLE}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║           🛡️  AIM - AWS Deployment Script 🛡️                  ║"
    echo "║                                                                ║"
    echo "║                  Deploying to ECS Fargate                     ║"
    echo "║                                                                ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_step() {
    echo -e "\n${CYAN}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

################################################################################
# Prerequisites Check
################################################################################

check_prerequisites() {
    print_step "Checking prerequisites..."

    # Check AWS CLI
    if ! command -v aws &> /dev/null; then
        print_error "AWS CLI not found"
        print_info "Install: https://aws.amazon.com/cli/"
        exit 1
    fi
    print_success "AWS CLI found: $(aws --version | cut -d' ' -f1)"

    # Check credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        print_error "AWS credentials not configured"
        print_info "Run: aws configure"
        exit 1
    fi
    print_success "AWS credentials configured"

    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker not found"
        exit 1
    fi
    print_success "Docker found"

    # Check jq
    if ! command -v jq &> /dev/null; then
        print_error "jq not found (required for JSON parsing)"
        print_info "Install: brew install jq (macOS) or apt-get install jq (Linux)"
        exit 1
    fi
    print_success "jq found"

    # Get AWS account ID
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query "Account" --output text)
    print_success "AWS Account ID: $AWS_ACCOUNT_ID"

    print_success "All prerequisites met!"
}

################################################################################
# Setup VPC and Networking
################################################################################

setup_vpc() {
    print_step "Setting up VPC and networking..."

    # Check if VPC exists
    VPC_ID=$(aws ec2 describe-vpcs \
        --filters "Name=tag:Name,Values=$VPC_NAME" \
        --query "Vpcs[0].VpcId" \
        --output text 2>/dev/null)

    if [ "$VPC_ID" != "None" ] && [ -n "$VPC_ID" ]; then
        print_info "VPC already exists: $VPC_ID"
    else
        print_info "Creating VPC..."
        VPC_ID=$(aws ec2 create-vpc \
            --cidr-block 10.0.0.0/16 \
            --query "Vpc.VpcId" \
            --output text)

        aws ec2 create-tags \
            --resources "$VPC_ID" \
            --tags "Key=Name,Value=$VPC_NAME"

        # Enable DNS hostnames
        aws ec2 modify-vpc-attribute \
            --vpc-id "$VPC_ID" \
            --enable-dns-hostnames

        print_success "VPC created: $VPC_ID"

        # Create Internet Gateway
        print_info "Creating Internet Gateway..."
        IGW_ID=$(aws ec2 create-internet-gateway \
            --query "InternetGateway.InternetGatewayId" \
            --output text)

        aws ec2 attach-internet-gateway \
            --vpc-id "$VPC_ID" \
            --internet-gateway-id "$IGW_ID"

        aws ec2 create-tags \
            --resources "$IGW_ID" \
            --tags "Key=Name,Value=${VPC_NAME}-igw"

        print_success "Internet Gateway created"

        # Create public subnets
        print_info "Creating subnets..."
        SUBNET_PUBLIC_1=$(aws ec2 create-subnet \
            --vpc-id "$VPC_ID" \
            --cidr-block 10.0.1.0/24 \
            --availability-zone "${AWS_REGION}a" \
            --query "Subnet.SubnetId" \
            --output text)

        SUBNET_PUBLIC_2=$(aws ec2 create-subnet \
            --vpc-id "$VPC_ID" \
            --cidr-block 10.0.2.0/24 \
            --availability-zone "${AWS_REGION}b" \
            --query "Subnet.SubnetId" \
            --output text)

        # Create private subnets
        SUBNET_PRIVATE_1=$(aws ec2 create-subnet \
            --vpc-id "$VPC_ID" \
            --cidr-block 10.0.10.0/24 \
            --availability-zone "${AWS_REGION}a" \
            --query "Subnet.SubnetId" \
            --output text)

        SUBNET_PRIVATE_2=$(aws ec2 create-subnet \
            --vpc-id "$VPC_ID" \
            --cidr-block 10.0.11.0/24 \
            --availability-zone "${AWS_REGION}b" \
            --query "Subnet.SubnetId" \
            --output text)

        # Tag subnets
        aws ec2 create-tags \
            --resources "$SUBNET_PUBLIC_1" "$SUBNET_PUBLIC_2" \
            --tags "Key=Name,Value=${VPC_NAME}-public"

        aws ec2 create-tags \
            --resources "$SUBNET_PRIVATE_1" "$SUBNET_PRIVATE_2" \
            --tags "Key=Name,Value=${VPC_NAME}-private"

        # Create route table
        ROUTE_TABLE_ID=$(aws ec2 create-route-table \
            --vpc-id "$VPC_ID" \
            --query "RouteTable.RouteTableId" \
            --output text)

        aws ec2 create-route \
            --route-table-id "$ROUTE_TABLE_ID" \
            --destination-cidr-block 0.0.0.0/0 \
            --gateway-id "$IGW_ID"

        # Associate route table with public subnets
        aws ec2 associate-route-table \
            --route-table-id "$ROUTE_TABLE_ID" \
            --subnet-id "$SUBNET_PUBLIC_1"

        aws ec2 associate-route-table \
            --route-table-id "$ROUTE_TABLE_ID" \
            --subnet-id "$SUBNET_PUBLIC_2"

        print_success "Subnets created"
    fi

    # Store subnet IDs
    SUBNET_IDS=$(aws ec2 describe-subnets \
        --filters "Name=vpc-id,Values=$VPC_ID" "Name=tag:Name,Values=${VPC_NAME}-public" \
        --query "Subnets[*].SubnetId" \
        --output text)

    print_success "VPC setup complete"
}

################################################################################
# Setup ECR Repositories
################################################################################

setup_ecr() {
    print_step "Setting up ECR repositories..."

    # Create backend repository
    if aws ecr describe-repositories --repository-names "$ECR_REPO_BACKEND" --region "$AWS_REGION" &> /dev/null; then
        print_info "Backend ECR repository already exists"
    else
        print_info "Creating backend ECR repository..."
        aws ecr create-repository \
            --repository-name "$ECR_REPO_BACKEND" \
            --region "$AWS_REGION"
        print_success "Backend repository created"
    fi

    # Create frontend repository
    if aws ecr describe-repositories --repository-names "$ECR_REPO_FRONTEND" --region "$AWS_REGION" &> /dev/null; then
        print_info "Frontend ECR repository already exists"
    else
        print_info "Creating frontend ECR repository..."
        aws ecr create-repository \
            --repository-name "$ECR_REPO_FRONTEND" \
            --region "$AWS_REGION"
        print_success "Frontend repository created"
    fi

    # Login to ECR
    print_info "Logging in to ECR..."
    aws ecr get-login-password --region "$AWS_REGION" | \
        docker login --username AWS --password-stdin \
        "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

    ECR_BACKEND_URI="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO_BACKEND}"
    ECR_FRONTEND_URI="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO_FRONTEND}"

    print_success "ECR configured"
}

################################################################################
# Setup RDS (PostgreSQL)
################################################################################

setup_rds() {
    print_step "Setting up RDS PostgreSQL..."

    # Create security group for RDS
    RDS_SG_ID=$(aws ec2 create-security-group \
        --group-name "${RDS_INSTANCE}-sg" \
        --description "Security group for AIM RDS" \
        --vpc-id "$VPC_ID" \
        --query "GroupId" \
        --output text 2>/dev/null) || \
    RDS_SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=${RDS_INSTANCE}-sg" \
        --query "SecurityGroups[0].GroupId" \
        --output text)

    # Allow PostgreSQL from VPC
    aws ec2 authorize-security-group-ingress \
        --group-id "$RDS_SG_ID" \
        --protocol tcp \
        --port 5432 \
        --cidr 10.0.0.0/16 2>/dev/null || true

    # Create DB subnet group
    SUBNET_IDS_ARRAY=($SUBNET_IDS)
    aws rds create-db-subnet-group \
        --db-subnet-group-name "${RDS_INSTANCE}-subnet-group" \
        --db-subnet-group-description "Subnet group for AIM RDS" \
        --subnet-ids ${SUBNET_IDS_ARRAY[@]} 2>/dev/null || true

    # Check if RDS instance exists
    if aws rds describe-db-instances --db-instance-identifier "$RDS_INSTANCE" &> /dev/null; then
        print_info "RDS instance already exists"
    else
        print_info "Creating RDS instance: $RDS_INSTANCE"
        print_info "This may take 10-15 minutes..."

        # Generate password
        RDS_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)

        aws rds create-db-instance \
            --db-instance-identifier "$RDS_INSTANCE" \
            --db-instance-class db.t3.micro \
            --engine postgres \
            --engine-version 16.1 \
            --master-username aimadmin \
            --master-user-password "$RDS_PASSWORD" \
            --allocated-storage 20 \
            --vpc-security-group-ids "$RDS_SG_ID" \
            --db-subnet-group-name "${RDS_INSTANCE}-subnet-group" \
            --db-name identity \
            --backup-retention-period 7 \
            --no-publicly-accessible

        print_info "Waiting for RDS instance to be available..."
        aws rds wait db-instance-available --db-instance-identifier "$RDS_INSTANCE"

        print_success "RDS instance created"
    fi

    # Get RDS endpoint
    RDS_ENDPOINT=$(aws rds describe-db-instances \
        --db-instance-identifier "$RDS_INSTANCE" \
        --query "DBInstances[0].Endpoint.Address" \
        --output text)

    DATABASE_URL="postgresql://aimadmin:${RDS_PASSWORD}@${RDS_ENDPOINT}:5432/identity"

    print_success "RDS configured"
}

################################################################################
# Setup ElastiCache (Redis)
################################################################################

setup_redis() {
    print_step "Setting up ElastiCache (Redis)..."

    # Create security group for Redis
    REDIS_SG_ID=$(aws ec2 create-security-group \
        --group-name "${REDIS_CLUSTER}-sg" \
        --description "Security group for AIM Redis" \
        --vpc-id "$VPC_ID" \
        --query "GroupId" \
        --output text 2>/dev/null) || \
    REDIS_SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=${REDIS_CLUSTER}-sg" \
        --query "SecurityGroups[0].GroupId" \
        --output text)

    # Allow Redis from VPC
    aws ec2 authorize-security-group-ingress \
        --group-id "$REDIS_SG_ID" \
        --protocol tcp \
        --port 6379 \
        --cidr 10.0.0.0/16 2>/dev/null || true

    # Create cache subnet group
    aws elasticache create-cache-subnet-group \
        --cache-subnet-group-name "${REDIS_CLUSTER}-subnet-group" \
        --cache-subnet-group-description "Subnet group for AIM Redis" \
        --subnet-ids ${SUBNET_IDS_ARRAY[@]} 2>/dev/null || true

    # Check if Redis cluster exists
    if aws elasticache describe-cache-clusters --cache-cluster-id "$REDIS_CLUSTER" &> /dev/null; then
        print_info "Redis cluster already exists"
    else
        print_info "Creating Redis cluster: $REDIS_CLUSTER"

        aws elasticache create-cache-cluster \
            --cache-cluster-id "$REDIS_CLUSTER" \
            --cache-node-type cache.t3.micro \
            --engine redis \
            --engine-version 7.0 \
            --num-cache-nodes 1 \
            --cache-subnet-group-name "${REDIS_CLUSTER}-subnet-group" \
            --security-group-ids "$REDIS_SG_ID"

        print_info "Waiting for Redis cluster to be available..."
        aws elasticache wait cache-cluster-available --cache-cluster-id "$REDIS_CLUSTER"

        print_success "Redis cluster created"
    fi

    # Get Redis endpoint
    REDIS_ENDPOINT=$(aws elasticache describe-cache-clusters \
        --cache-cluster-id "$REDIS_CLUSTER" \
        --show-cache-node-info \
        --query "CacheClusters[0].CacheNodes[0].Endpoint.Address" \
        --output text)

    REDIS_URL="redis://${REDIS_ENDPOINT}:6379/0"

    print_success "Redis configured"
}

################################################################################
# Build and Push Docker Images
################################################################################

build_and_push_images() {
    print_step "Building and pushing Docker images..."

    # Build and push backend
    print_info "Building backend image..."
    cd apps/backend
    docker build -t "${ECR_BACKEND_URI}:latest" -f Dockerfile .

    print_info "Pushing backend image..."
    docker push "${ECR_BACKEND_URI}:latest"
    print_success "Backend image pushed"

    # Build and push frontend
    cd ../web
    print_info "Building frontend image..."
    docker build -t "${ECR_FRONTEND_URI}:latest" -f Dockerfile .

    print_info "Pushing frontend image..."
    docker push "${ECR_FRONTEND_URI}:latest"
    print_success "Frontend image pushed"

    cd ../..
}

################################################################################
# Setup ECS
################################################################################

setup_ecs_cluster() {
    print_step "Setting up ECS cluster..."

    # Create ECS cluster
    if aws ecs describe-clusters --clusters "$CLUSTER_NAME" --query "clusters[0].clusterName" --output text | grep -q "$CLUSTER_NAME"; then
        print_info "ECS cluster already exists"
    else
        print_info "Creating ECS cluster: $CLUSTER_NAME"
        aws ecs create-cluster --cluster-name "$CLUSTER_NAME"
        print_success "ECS cluster created"
    fi
}

create_task_execution_role() {
    print_step "Creating IAM role for ECS task execution..."

    ROLE_NAME="${SERVICE_NAME}-execution-role"

    # Create role
    aws iam create-role \
        --role-name "$ROLE_NAME" \
        --assume-role-policy-document '{
            "Version": "2012-10-17",
            "Statement": [{
                "Effect": "Allow",
                "Principal": {"Service": "ecs-tasks.amazonaws.com"},
                "Action": "sts:AssumeRole"
            }]
        }' 2>/dev/null || true

    # Attach policies
    aws iam attach-role-policy \
        --role-name "$ROLE_NAME" \
        --policy-arn "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy" 2>/dev/null || true

    EXECUTION_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${ROLE_NAME}"

    print_success "Task execution role created"
}

deploy_backend_task() {
    print_step "Deploying backend ECS task..."

    # Generate JWT secret
    JWT_SECRET=$(openssl rand -hex 32)

    # Register task definition
    print_info "Registering backend task definition..."
    TASK_DEF=$(cat <<EOF
{
    "family": "${SERVICE_NAME}-backend",
    "networkMode": "awsvpc",
    "requiresCompatibilities": ["FARGATE"],
    "cpu": "1024",
    "memory": "2048",
    "executionRoleArn": "$EXECUTION_ROLE_ARN",
    "containerDefinitions": [{
        "name": "backend",
        "image": "${ECR_BACKEND_URI}:latest",
        "portMappings": [{"containerPort": 8080, "protocol": "tcp"}],
        "environment": [
            {"name": "SERVER_PORT", "value": "8080"},
            {"name": "ENVIRONMENT", "value": "production"},
            {"name": "JWT_SECRET", "value": "$JWT_SECRET"},
            {"name": "DATABASE_URL", "value": "$DATABASE_URL"},
            {"name": "REDIS_URL", "value": "$REDIS_URL"}
        ],
        "logConfiguration": {
            "logDriver": "awslogs",
            "options": {
                "awslogs-group": "/ecs/${SERVICE_NAME}-backend",
                "awslogs-region": "$AWS_REGION",
                "awslogs-stream-prefix": "ecs",
                "awslogs-create-group": "true"
            }
        }
    }]
}
EOF
)

    echo "$TASK_DEF" | aws ecs register-task-definition --cli-input-json file:///dev/stdin > /dev/null

    # Create security group for backend
    BACKEND_SG_ID=$(aws ec2 create-security-group \
        --group-name "${SERVICE_NAME}-backend-sg" \
        --description "Security group for AIM backend" \
        --vpc-id "$VPC_ID" \
        --query "GroupId" \
        --output text 2>/dev/null) || \
    BACKEND_SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=${SERVICE_NAME}-backend-sg" \
        --query "SecurityGroups[0].GroupId" \
        --output text)

    # Allow port 8080 from anywhere (for now)
    aws ec2 authorize-security-group-ingress \
        --group-id "$BACKEND_SG_ID" \
        --protocol tcp \
        --port 8080 \
        --cidr 0.0.0.0/0 2>/dev/null || true

    # Create or update service
    print_info "Creating backend ECS service..."
    aws ecs create-service \
        --cluster "$CLUSTER_NAME" \
        --service-name "${SERVICE_NAME}-backend" \
        --task-definition "${SERVICE_NAME}-backend" \
        --desired-count 1 \
        --launch-type FARGATE \
        --network-configuration "awsvpcConfiguration={subnets=[$SUBNET_IDS],securityGroups=[$BACKEND_SG_ID],assignPublicIp=ENABLED}" 2>/dev/null || \
    aws ecs update-service \
        --cluster "$CLUSTER_NAME" \
        --service "${SERVICE_NAME}-backend" \
        --force-new-deployment > /dev/null

    print_success "Backend service deployed"
}

deploy_frontend_task() {
    print_step "Deploying frontend ECS task..."

    # Register task definition
    print_info "Registering frontend task definition..."
    TASK_DEF=$(cat <<EOF
{
    "family": "${SERVICE_NAME}-frontend",
    "networkMode": "awsvpc",
    "requiresCompatibilities": ["FARGATE"],
    "cpu": "512",
    "memory": "1024",
    "executionRoleArn": "$EXECUTION_ROLE_ARN",
    "containerDefinitions": [{
        "name": "frontend",
        "image": "${ECR_FRONTEND_URI}:latest",
        "portMappings": [{"containerPort": 3000, "protocol": "tcp"}],
        "environment": [
            {"name": "NEXT_PUBLIC_API_URL", "value": "http://backend:8080"}
        ],
        "logConfiguration": {
            "logDriver": "awslogs",
            "options": {
                "awslogs-group": "/ecs/${SERVICE_NAME}-frontend",
                "awslogs-region": "$AWS_REGION",
                "awslogs-stream-prefix": "ecs",
                "awslogs-create-group": "true"
            }
        }
    }]
}
EOF
)

    echo "$TASK_DEF" | aws ecs register-task-definition --cli-input-json file:///dev/stdin > /dev/null

    # Create security group for frontend
    FRONTEND_SG_ID=$(aws ec2 create-security-group \
        --group-name "${SERVICE_NAME}-frontend-sg" \
        --description "Security group for AIM frontend" \
        --vpc-id "$VPC_ID" \
        --query "GroupId" \
        --output text 2>/dev/null) || \
    FRONTEND_SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=${SERVICE_NAME}-frontend-sg" \
        --query "SecurityGroups[0].GroupId" \
        --output text)

    # Allow port 3000 from anywhere
    aws ec2 authorize-security-group-ingress \
        --group-id "$FRONTEND_SG_ID" \
        --protocol tcp \
        --port 3000 \
        --cidr 0.0.0.0/0 2>/dev/null || true

    # Create or update service
    print_info "Creating frontend ECS service..."
    aws ecs create-service \
        --cluster "$CLUSTER_NAME" \
        --service-name "${SERVICE_NAME}-frontend" \
        --task-definition "${SERVICE_NAME}-frontend" \
        --desired-count 1 \
        --launch-type FARGATE \
        --network-configuration "awsvpcConfiguration={subnets=[$SUBNET_IDS],securityGroups=[$FRONTEND_SG_ID],assignPublicIp=ENABLED}" 2>/dev/null || \
    aws ecs update-service \
        --cluster "$CLUSTER_NAME" \
        --service "${SERVICE_NAME}-frontend" \
        --force-new-deployment > /dev/null

    print_success "Frontend service deployed"
}

################################################################################
# Post-Deployment
################################################################################

get_service_urls() {
    print_step "Getting service URLs..."

    # Wait for tasks to start
    print_info "Waiting for tasks to start..."
    sleep 30

    # Get backend task public IP
    BACKEND_TASK_ARN=$(aws ecs list-tasks \
        --cluster "$CLUSTER_NAME" \
        --service-name "${SERVICE_NAME}-backend" \
        --query "taskArns[0]" \
        --output text)

    if [ "$BACKEND_TASK_ARN" != "None" ]; then
        BACKEND_ENI=$(aws ecs describe-tasks \
            --cluster "$CLUSTER_NAME" \
            --tasks "$BACKEND_TASK_ARN" \
            --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
            --output text)

        BACKEND_IP=$(aws ec2 describe-network-interfaces \
            --network-interface-ids "$BACKEND_ENI" \
            --query "NetworkInterfaces[0].Association.PublicIp" \
            --output text)

        BACKEND_URL="http://${BACKEND_IP}:8080"
    fi

    # Get frontend task public IP
    FRONTEND_TASK_ARN=$(aws ecs list-tasks \
        --cluster "$CLUSTER_NAME" \
        --service-name "${SERVICE_NAME}-frontend" \
        --query "taskArns[0]" \
        --output text)

    if [ "$FRONTEND_TASK_ARN" != "None" ]; then
        FRONTEND_ENI=$(aws ecs describe-tasks \
            --cluster "$CLUSTER_NAME" \
            --tasks "$FRONTEND_TASK_ARN" \
            --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
            --output text)

        FRONTEND_IP=$(aws ec2 describe-network-interfaces \
            --network-interface-ids "$FRONTEND_ENI" \
            --query "NetworkInterfaces[0].Association.PublicIp" \
            --output text)

        FRONTEND_URL="http://${FRONTEND_IP}:3000"
    fi
}

print_deployment_summary() {
    print_step "Deployment Summary"

    echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  🎉 AIM Deployed to AWS Successfully! 🎉                      ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"

    echo -e "${CYAN}📊 Deployment Information:${NC}"
    echo -e "  ${BLUE}•${NC} Region:             $AWS_REGION"
    echo -e "  ${BLUE}•${NC} ECS Cluster:        $CLUSTER_NAME"
    echo -e "  ${BLUE}•${NC} Frontend URL:       $FRONTEND_URL"
    echo -e "  ${BLUE}•${NC} Backend URL:        $BACKEND_URL"

    echo -e "\n${CYAN}🔧 AWS Resources:${NC}"
    echo -e "  ${BLUE}•${NC} VPC ID:             $VPC_ID"
    echo -e "  ${BLUE}•${NC} RDS Instance:       $RDS_INSTANCE"
    echo -e "  ${BLUE}•${NC} Redis Cluster:      $REDIS_CLUSTER"
    echo -e "  ${BLUE}•${NC} ECR Repositories:   $ECR_REPO_BACKEND, $ECR_REPO_FRONTEND"

    echo -e "\n${CYAN}📚 Next Steps:${NC}"
    echo -e "  1. Set up Application Load Balancer for better routing"
    echo -e "  2. Configure Route53 for custom domain"
    echo -e "  3. Set up CloudWatch dashboards"
    echo -e "  4. Configure AWS Certificate Manager for HTTPS"
    echo -e "  5. Set up auto-scaling policies"

    echo -e "\n${CYAN}📝 Management Commands:${NC}"
    echo -e "  ${BLUE}•${NC} View logs:          aws logs tail /ecs/${SERVICE_NAME}-backend --follow"
    echo -e "  ${BLUE}•${NC} Scale service:      aws ecs update-service --cluster $CLUSTER_NAME --service ${SERVICE_NAME}-backend --desired-count 2"
    echo -e "  ${BLUE}•${NC} Update service:     aws ecs update-service --cluster $CLUSTER_NAME --service ${SERVICE_NAME}-backend --force-new-deployment"
    echo -e "  ${BLUE}•${NC} Delete cluster:     aws ecs delete-cluster --cluster $CLUSTER_NAME"

    echo -e "\n${GREEN}🚀 AIM is now running on AWS!${NC}\n"
}

################################################################################
# Main
################################################################################

main() {
    print_header

    print_info "Starting AWS deployment..."
    print_info "Region: $AWS_REGION"
    print_info "Cluster: $CLUSTER_NAME"

    check_prerequisites
    setup_vpc
    setup_ecr
    setup_rds
    setup_redis
    build_and_push_images
    setup_ecs_cluster
    create_task_execution_role
    deploy_backend_task
    deploy_frontend_task
    get_service_urls
    print_deployment_summary
}

# Run main
main "$@"
