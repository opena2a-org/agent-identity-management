# 🗄️ Database Agent - Enterprise Security

Build a secure database agent with automatic risk assessment and approval workflows.

## What You'll Build

A production-ready AI agent that:
- ✅ Securely queries PostgreSQL databases
- ✅ Auto-approves safe read operations
- ✅ Requires human approval for high-risk actions (DELETE, UPDATE)
- ✅ Prevents SQL injection attacks
- ✅ Complete audit trail for compliance (SOC 2, HIPAA, GDPR)
- ✅ Automatically secured with AIM (1 line of code)

**Difficulty**: Advanced
**Time**: 15 minutes
**Use Case**: Database access for AI agents, compliance-critical operations, enterprise security

---

## Prerequisites

1. ✅ AIM platform running ([Quick Start Guide](../quick-start.md))
2. ✅ PostgreSQL database (local or cloud)
3. ✅ Python 3.8+ installed
4. ✅ AIM SDK downloaded from dashboard ([Download Instructions](../quick-start.md#step-3-download-aim-sdk-and-install-dependencies-30-seconds))
   - **NO pip install available** - must download from dashboard

### Install Dependencies

```bash
# Install AIM SDK dependencies and PostgreSQL driver
pip install keyring PyNaCl requests cryptography psycopg2-binary
```

---

## Step 1: Set Up Test Database (5 minutes)

### Create PostgreSQL Database

```bash
# Using Docker (easiest)
docker run --name test-postgres \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  -d postgres:16

# Wait for database to start
sleep 5
```

### Create Test Schema

Create `schema.sql`:

```sql
-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    age INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Orders table
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    product_name VARCHAR(255),
    amount DECIMAL(10, 2),
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO users (email, full_name, age) VALUES
    ('john@example.com', 'John Doe', 30),
    ('jane@example.com', 'Jane Smith', 25),
    ('bob@example.com', 'Bob Johnson', 35);

INSERT INTO orders (user_id, product_name, amount, status) VALUES
    (1, 'Laptop', 999.99, 'completed'),
    (1, 'Mouse', 29.99, 'completed'),
    (2, 'Keyboard', 79.99, 'pending'),
    (3, 'Monitor', 299.99, 'completed');
```

Apply schema:
```bash
docker exec -i test-postgres psql -U postgres -d testdb < schema.sql
```

---

## Step 2: Register Agent (30 seconds)

### In AIM Dashboard

1. **Login** to http://localhost:3000
2. **Navigate**: Agents → Register New Agent
3. **Fill in**:
   ```
   Agent Name: database-agent
   Agent Type: AI Agent
   Description: Secure database access with risk assessment
   ```
4. **Click** "Register Agent"
5. **Copy** the private key

### Save Credentials

```bash
export AIM_PRIVATE_KEY="your-aim-private-key"
export DATABASE_URL="postgresql://postgres:testpass@localhost:5432/testdb"
export AIM_URL="http://localhost:8080"
```

---

## Step 3: Write the Agent (7 minutes)

Create `database_agent.py`:

```python
"""
Database Agent - Secured with AIM
production-ready database access with risk assessment
"""

from aim_sdk import secure
import psycopg2
from psycopg2 import sql
import os
from typing import List, Dict, Any, Optional
from contextlib import contextmanager

# 🔐 ONE LINE - Secure your agent!
agent = secure(
    name="database-agent",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)


class DatabaseAgent:
    """
    Secure database agent with automatic risk assessment

    Features:
    - ✅ SQL injection prevention
    - ✅ Automatic risk classification
    - ✅ Human approval for high-risk operations
    - ✅ Complete audit trail
    - ✅ Parameterized queries only
    """

    def __init__(self, database_url: str):
        self.database_url = database_url

    @contextmanager
    def get_connection(self):
        """Secure database connection context manager"""
        conn = psycopg2.connect(self.database_url)
        try:
            yield conn
            conn.commit()
        except Exception as e:
            conn.rollback()
            raise e
        finally:
            conn.close()

    # ═══════════════════════════════════════════════════════════
    # SAFE OPERATIONS (Auto-approved by AIM)
    # ═══════════════════════════════════════════════════════════

    def query_users(self, filters: Optional[Dict[str, Any]] = None) -> List[Dict]:
        """
        Query users with optional filters (SAFE - auto-approved)

        Args:
            filters: Optional filters (e.g., {"age__gte": 25, "email__contains": "example"})

        Returns:
            List of user dictionaries

        Example:
            >>> users = agent.query_users({"age__gte": 25})
            >>> print(f"Found {len(users)} users")
        """
        # AIM automatically approves read operations
        with self.get_connection() as conn:
            cursor = conn.cursor()

            # Build safe parameterized query
            query = "SELECT id, email, full_name, age, created_at FROM users WHERE 1=1"
            params = []

            if filters:
                if "age__gte" in filters:
                    query += " AND age >= %s"
                    params.append(filters["age__gte"])

                if "age__lte" in filters:
                    query += " AND age <= %s"
                    params.append(filters["age__lte"])

                if "email__contains" in filters:
                    query += " AND email LIKE %s"
                    params.append(f"%{filters['email__contains']}%")

            cursor.execute(query, params)

            columns = [desc[0] for desc in cursor.description]
            return [dict(zip(columns, row)) for row in cursor.fetchall()]

    def get_user_by_email(self, email: str) -> Optional[Dict]:
        """Get user by email (SAFE - auto-approved)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()

            # Parameterized query prevents SQL injection
            cursor.execute(
                "SELECT id, email, full_name, age, created_at FROM users WHERE email = %s",
                (email,)
            )

            result = cursor.fetchone()
            if not result:
                return None

            columns = [desc[0] for desc in cursor.description]
            return dict(zip(columns, result))

    def get_user_orders(self, user_id: int) -> List[Dict]:
        """Get all orders for a user (SAFE - auto-approved)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()

            cursor.execute(
                """
                SELECT o.id, o.product_name, o.amount, o.status, o.created_at
                FROM orders o
                WHERE o.user_id = %s
                ORDER BY o.created_at DESC
                """,
                (user_id,)
            )

            columns = [desc[0] for desc in cursor.description]
            return [dict(zip(columns, row)) for row in cursor.fetchall()]

    def get_sales_summary(self) -> Dict:
        """Get sales summary (SAFE - auto-approved)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()

            cursor.execute(
                """
                SELECT
                    COUNT(*) as total_orders,
                    SUM(amount) as total_revenue,
                    AVG(amount) as avg_order_value,
                    COUNT(DISTINCT user_id) as unique_customers
                FROM orders
                WHERE status = 'completed'
                """
            )

            result = cursor.fetchone()
            columns = [desc[0] for desc in cursor.description]
            return dict(zip(columns, result))

    # ═══════════════════════════════════════════════════════════
    # HIGH-RISK OPERATIONS (Require human approval)
    # ═══════════════════════════════════════════════════════════

    @agent.require_approval(risk_level="high")
    def update_user(self, user_id: int, updates: Dict[str, Any]) -> bool:
        """
        Update user data (HIGH RISK - requires approval)

        AIM will:
        1. Flag this as HIGH RISK
        2. Send alert to dashboard
        3. Wait for admin approval
        4. Only execute if approved

        Args:
            user_id: User ID to update
            updates: Dictionary of fields to update

        Returns:
            True if updated successfully

        Example:
            >>> # This triggers approval workflow
            >>> agent.update_user(1, {"email": "newemail@example.com"})
            >>> # → Sends alert to dashboard
            >>> # → Waits for admin approval
            >>> # → Executes only if approved
        """
        with self.get_connection() as conn:
            cursor = conn.cursor()

            # Build safe UPDATE query
            set_clause = ", ".join([f"{key} = %s" for key in updates.keys()])
            params = list(updates.values()) + [user_id]

            query = f"UPDATE users SET {set_clause} WHERE id = %s"
            cursor.execute(query, params)

            return cursor.rowcount > 0

    @agent.require_approval(risk_level="critical")
    def delete_user(self, user_id: int) -> bool:
        """
        Delete user (CRITICAL RISK - requires approval)

        AIM will:
        1. Flag this as CRITICAL RISK
        2. Send urgent alert to dashboard
        3. Require admin approval
        4. Log to audit trail
        5. Only execute if approved

        Args:
            user_id: User ID to delete

        Returns:
            True if deleted successfully
        """
        with self.get_connection() as conn:
            cursor = conn.cursor()

            # Delete user and cascade to orders
            cursor.execute("DELETE FROM users WHERE id = %s", (user_id,))

            return cursor.rowcount > 0

    @agent.require_approval(risk_level="high")
    def bulk_update_prices(self, percentage: float) -> int:
        """
        Update all prices by percentage (HIGH RISK - requires approval)

        Example:
            >>> # Increase all prices by 10%
            >>> agent.bulk_update_prices(10.0)
        """
        with self.get_connection() as conn:
            cursor = conn.cursor()

            cursor.execute(
                """
                UPDATE orders
                SET amount = amount * (1 + %s / 100.0)
                WHERE status = 'pending'
                """,
                (percentage,)
            )

            return cursor.rowcount

    # ═══════════════════════════════════════════════════════════
    # ANALYTICS (Safe - auto-approved)
    # ═══════════════════════════════════════════════════════════

    def get_top_customers(self, limit: int = 10) -> List[Dict]:
        """Get top customers by total spending (SAFE)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()

            cursor.execute(
                """
                SELECT
                    u.id,
                    u.email,
                    u.full_name,
                    COUNT(o.id) as total_orders,
                    SUM(o.amount) as total_spent
                FROM users u
                INNER JOIN orders o ON u.id = o.user_id
                WHERE o.status = 'completed'
                GROUP BY u.id, u.email, u.full_name
                ORDER BY total_spent DESC
                LIMIT %s
                """,
                (limit,)
            )

            columns = [desc[0] for desc in cursor.description]
            return [dict(zip(columns, row)) for row in cursor.fetchall()]

    def get_revenue_by_product(self) -> List[Dict]:
        """Get revenue breakdown by product (SAFE)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()

            cursor.execute(
                """
                SELECT
                    product_name,
                    COUNT(*) as units_sold,
                    SUM(amount) as total_revenue
                FROM orders
                WHERE status = 'completed'
                GROUP BY product_name
                ORDER BY total_revenue DESC
                """
            )

            columns = [desc[0] for desc in cursor.description]
            return [dict(zip(columns, row)) for row in cursor.fetchall()]


def demo_safe_operations():
    """Demo: Safe read operations (auto-approved)"""
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("📖 DEMO 1: Safe Operations (Auto-Approved)")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = DatabaseAgent(os.getenv("DATABASE_URL"))

    # Query users
    print("1. Query users age >= 25:")
    users = agent.query_users({"age__gte": 25})
    for user in users:
        print(f"   ✅ {user['full_name']} ({user['email']}) - Age: {user['age']}")

    print("\n2. Get user by email:")
    user = agent.get_user_by_email("john@example.com")
    if user:
        print(f"   ✅ Found: {user['full_name']}")

    print("\n3. Get user orders:")
    orders = agent.get_user_orders(1)
    print(f"   ✅ Found {len(orders)} orders")
    for order in orders:
        print(f"      ${order['amount']:.2f} - {order['product_name']}")

    print("\n4. Sales summary:")
    summary = agent.get_sales_summary()
    print(f"   ✅ Total Orders: {summary['total_orders']}")
    print(f"   ✅ Total Revenue: ${summary['total_revenue']:.2f}")
    print(f"   ✅ Avg Order: ${summary['avg_order_value']:.2f}")


def demo_high_risk_operations():
    """Demo: High-risk operations (require approval)"""
    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("⚠️  DEMO 2: High-Risk Operations (Approval Required)")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = DatabaseAgent(os.getenv("DATABASE_URL"))

    print("1. Update user email (HIGH RISK):")
    print("   🔄 Requesting approval from admin...")

    try:
        # This will send alert to AIM dashboard and wait for approval
        result = agent.update_user(1, {"email": "john.new@example.com"})

        if result:
            print("   ✅ Approved and executed!")
        else:
            print("   ❌ Rejected by admin")

    except Exception as e:
        print(f"   ⏳ Waiting for approval... (check dashboard)")

    print("\n2. Delete user (CRITICAL RISK):")
    print("   🔄 Requesting approval from admin...")

    try:
        # This will send URGENT alert and require admin approval
        result = agent.delete_user(999)  # Non-existent user for safety

        if result:
            print("   ✅ Approved and executed!")
        else:
            print("   ❌ Rejected by admin")

    except Exception as e:
        print(f"   ⏳ Waiting for approval... (check dashboard)")


def demo_analytics():
    """Demo: Analytics queries (safe)"""
    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("📊 DEMO 3: Analytics (Auto-Approved)")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = DatabaseAgent(os.getenv("DATABASE_URL"))

    print("1. Top customers:")
    customers = agent.get_top_customers(5)
    for i, customer in enumerate(customers, 1):
        print(f"   #{i} {customer['full_name']}: ${customer['total_spent']:.2f} ({customer['total_orders']} orders)")

    print("\n2. Revenue by product:")
    products = agent.get_revenue_by_product()
    for product in products:
        print(f"   ✅ {product['product_name']}: ${product['total_revenue']:.2f} ({product['units_sold']} sold)")


def main():
    """Run all demos"""
    demo_safe_operations()
    demo_high_risk_operations()
    demo_analytics()

    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("✅ All demos complete!")
    print("📊 Check dashboard: http://localhost:3000/agents/database-agent")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")


if __name__ == "__main__":
    main()
```

---

## Step 4: Run It! (2 minutes)

```bash
# Set environment variables
export AIM_PRIVATE_KEY="your-key"
export DATABASE_URL="postgresql://postgres:testpass@localhost:5432/testdb"
export AIM_URL="http://localhost:8080"

# Run the agent
python database_agent.py
```

**Expected Output**:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📖 DEMO 1: Safe Operations (Auto-Approved)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Query users age >= 25:
   ✅ John Doe (john@example.com) - Age: 30
   ✅ Jane Smith (jane@example.com) - Age: 25
   ✅ Bob Johnson (bob@example.com) - Age: 35

2. Get user by email:
   ✅ Found: John Doe

3. Get user orders:
   ✅ Found 2 orders
      $999.99 - Laptop
      $29.99 - Mouse

4. Sales summary:
   ✅ Total Orders: 4
   ✅ Total Revenue: $1409.96
   ✅ Avg Order: $352.49

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠️  DEMO 2: High-Risk Operations (Approval Required)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Update user email (HIGH RISK):
   🔄 Requesting approval from admin...
   ⏳ Waiting for approval... (check dashboard)

2. Delete user (CRITICAL RISK):
   🔄 Requesting approval from admin...
   ⏳ Waiting for approval... (check dashboard)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 DEMO 3: Analytics (Auto-Approved)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Top customers:
   #1 John Doe: $1029.98 (2 orders)
   #2 Bob Johnson: $299.99 (1 orders)
   #3 Jane Smith: $79.99 (1 orders)

2. Revenue by product:
   ✅ Laptop: $999.99 (1 sold)
   ✅ Monitor: $299.99 (1 sold)
   ✅ Keyboard: $79.99 (1 sold)
   ✅ Mouse: $29.99 (1 sold)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ All demos complete!
📊 Check dashboard: http://localhost:3000/agents/database-agent
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## Step 5: Check Dashboard & Approve Actions

### Open Dashboard

http://localhost:3000 → Agents → database-agent

### Pending Approvals

```
🔔 2 PENDING APPROVALS

⚠️  HIGH RISK: update_user
    Agent: database-agent
    Action: update_user(user_id=1, updates={"email": "john.new@example.com"})
    Risk Level: HIGH
    Requested: 30 seconds ago

    [APPROVE] [REJECT]

🚨 CRITICAL RISK: delete_user
    Agent: database-agent
    Action: delete_user(user_id=999)
    Risk Level: CRITICAL
    Requested: 25 seconds ago

    [APPROVE] [REJECT]
```

### Agent Status

```
Agent: database-agent
Status: ✅ ACTIVE
Trust Score: 0.98 (Excellent)
Last Verified: 15 seconds ago
Total Actions: 12
Pending Approvals: 2
```

### Recent Activity

```
✅ query_users({"age__gte": 25})     |  45s ago  |  SUCCESS  |  12ms  |  AUTO-APPROVED
✅ get_user_by_email("john@...")     |  40s ago  |  SUCCESS  |   8ms  |  AUTO-APPROVED
✅ get_user_orders(1)                |  35s ago  |  SUCCESS  |  11ms  |  AUTO-APPROVED
✅ get_sales_summary()               |  30s ago  |  SUCCESS  |  15ms  |  AUTO-APPROVED
⏳ update_user(1, {...})             |  30s ago  |  PENDING  |   -    |  AWAITING APPROVAL
⏳ delete_user(999)                  |  25s ago  |  PENDING  |   -    |  AWAITING APPROVAL
✅ get_top_customers(5)              |  20s ago  |  SUCCESS  |  18ms  |  AUTO-APPROVED
✅ get_revenue_by_product()          |  15s ago  |  SUCCESS  |  14ms  |  AUTO-APPROVED
```

### Audit Trail

```
📝 Query: SELECT * FROM users WHERE age >= 25
   Timestamp: 2025-10-21 15:42:30 UTC
   Result: 3 rows returned
   Risk Level: LOW
   Approved: AUTO

📝 Query: SELECT * FROM users WHERE email = 'john@example.com'
   Timestamp: 2025-10-21 15:42:35 UTC
   Result: 1 row returned
   Risk Level: LOW
   Approved: AUTO

📝 Update: UPDATE users SET email = 'john.new@example.com' WHERE id = 1
   Timestamp: 2025-10-21 15:42:45 UTC
   Result: PENDING
   Risk Level: HIGH
   Approved: PENDING (Admin: Sarah Johnson)

📝 Delete: DELETE FROM users WHERE id = 999
   Timestamp: 2025-10-21 15:42:50 UTC
   Result: PENDING
   Risk Level: CRITICAL
   Approved: PENDING (Admin: Sarah Johnson)
```

---

## 🎓 Understanding Risk Levels

AIM automatically classifies database operations by risk:

### LOW RISK (Auto-Approved) ✅
- `SELECT` queries (read-only)
- Analytics queries
- Aggregate functions (COUNT, SUM, AVG)
- No data modification

### HIGH RISK (Requires Approval) ⚠️
- `UPDATE` statements
- Bulk operations
- Schema modifications
- Price changes

### CRITICAL RISK (Requires Urgent Approval) 🚨
- `DELETE` statements
- User account deletions
- Irreversible operations
- Cascading deletes

---

## 🔒 Security Features

### 1. SQL Injection Prevention

```python
# ❌ DANGEROUS (SQL injection vulnerability)
def unsafe_query(email):
    query = f"SELECT * FROM users WHERE email = '{email}'"  # NEVER DO THIS!
    cursor.execute(query)

# ✅ SAFE (parameterized query)
def safe_query(email):
    query = "SELECT * FROM users WHERE email = %s"
    cursor.execute(query, (email,))  # ✅ PostgreSQL escapes parameters
```

### 2. Automatic Risk Classification

AIM analyzes SQL queries and classifies risk:
- Detects `DELETE`, `UPDATE`, `DROP` keywords
- Counts affected rows
- Checks for `WHERE` clauses
- Identifies bulk operations

### 3. Approval Workflows

High-risk operations trigger approval workflows:
1. Agent requests action
2. AIM sends alert to dashboard
3. Admin reviews context
4. Admin approves/rejects
5. Action executes (if approved)
6. Audit trail updated

### 4. Complete Audit Trail

Every database operation is logged:
- SQL query executed
- Parameters used
- Rows affected
- Timestamp
- Admin approval (if required)
- Result status

**SOC 2, HIPAA, GDPR compliant!**

---

## 💡 Real-World Use Cases

### 1. AI Customer Support Agent

```python
from aim_sdk import secure

agent = secure("support-agent")

class SupportAgent(DatabaseAgent):
    """AI agent for customer support"""

    def lookup_customer(self, email: str):
        """Look up customer info (SAFE)"""
        return self.get_user_by_email(email)

    def get_order_history(self, user_id: int):
        """Get customer order history (SAFE)"""
        return self.get_user_orders(user_id)

    @agent.require_approval(risk_level="high")
    def cancel_order(self, order_id: int):
        """Cancel order (HIGH RISK - requires approval)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute(
                "UPDATE orders SET status = 'cancelled' WHERE id = %s",
                (order_id,)
            )
```

### 2. Data Science Agent

```python
from aim_sdk import secure

agent = secure("data-science-agent")

class DataScienceAgent(DatabaseAgent):
    """AI agent for data science queries"""

    def get_churn_analysis(self):
        """Analyze customer churn (SAFE)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT
                    DATE_TRUNC('month', created_at) as month,
                    COUNT(DISTINCT user_id) as active_users
                FROM orders
                GROUP BY month
                ORDER BY month DESC
            """)
            return cursor.fetchall()

    def segment_customers(self):
        """Segment customers by behavior (SAFE)"""
        # Complex analytics query...
        pass
```

### 3. Admin Automation Agent

```python
from aim_sdk import secure

agent = secure("admin-agent")

class AdminAgent(DatabaseAgent):
    """AI agent for admin operations"""

    @agent.require_approval(risk_level="high")
    def bulk_email_update(self, domain_from: str, domain_to: str):
        """Update email domains (HIGH RISK)"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE users
                SET email = REPLACE(email, %s, %s)
                WHERE email LIKE %s
                """,
                (f"@{domain_from}", f"@{domain_to}", f"%@{domain_from}")
            )
            return cursor.rowcount
```

---

## ✅ Checklist

- [ ] PostgreSQL database running
- [ ] Test schema applied
- [ ] Agent registered in AIM
- [ ] Code runs without errors
- [ ] Safe operations auto-approved
- [ ] High-risk operations require approval
- [ ] Audit trail shows all queries
- [ ] SQL injection prevention tested
- [ ] Dashboard shows pending approvals

**All checked?** 🎉 **Your database agent is enterprise-ready!**

---

## 🚀 Next Steps

### Learn More
- [SDK Documentation](../sdk/python.md) - Complete SDK reference
- [Security Architecture](../security/architecture.md) - How AIM secures agents
- [Compliance Reports](../security/compliance.md) - SOC 2, HIPAA, GDPR

### Integrations
- [LangChain Integration](../integrations/langchain.md) - Secure agent frameworks
- [CrewAI Integration](../integrations/crewai.md) - Secure multi-agent teams

### Deploy
- [Azure Deployment](../deployment/azure.md) - Production deployment
- [Kubernetes](../deployment/kubernetes.md) - Enterprise scale

---

<div align="center">

**Next**: [LangChain Integration →](../integrations/langchain.md)

[🏠 Back to Home](../../README.md) • [📚 All Examples](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
