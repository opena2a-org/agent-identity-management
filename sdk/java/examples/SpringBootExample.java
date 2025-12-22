package org.opena2a.aim.examples;

import org.opena2a.aim.annotations.SecureAction;
import org.opena2a.aim.annotations.SecureActionAspect;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.RiskLevel;
import org.opena2a.aim.exceptions.ActionDeniedException;

import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/**
 * Example demonstrating AIM SDK integration with Spring Boot.
 *
 * <p>This example shows a typical Spring Boot setup with:</p>
 * <ul>
 *   <li>Configuration class with @EnableAspectJAutoProxy</li>
 *   <li>Service layer with @SecureAction annotations</li>
 *   <li>Controller layer calling secured services</li>
 * </ul>
 *
 * <p>To use in a real Spring Boot application:</p>
 * <ol>
 *   <li>Add AIM SDK dependency to pom.xml</li>
 *   <li>Create SecurityConfig class (see below)</li>
 *   <li>Annotate service methods with @SecureAction</li>
 * </ol>
 *
 * <p>Example pom.xml additions:</p>
 * <pre>{@code
 * <dependency>
 *     <groupId>org.opena2a</groupId>
 *     <artifactId>aim-sdk</artifactId>
 *     <version>1.0.0</version>
 * </dependency>
 * <dependency>
 *     <groupId>org.springframework.boot</groupId>
 *     <artifactId>spring-boot-starter-aop</artifactId>
 * </dependency>
 * }</pre>
 */
public class SpringBootExample {

    public static void main(String[] args) {
        System.out.println("=== AIM Spring Boot Integration Example ===\n");
        System.out.println("This example demonstrates the code structure for Spring Boot.");
        System.out.println("In a real application, Spring would manage these beans.\n");

        try {
            // Simulate Spring Boot startup
            System.out.println("1. Simulating Spring Boot startup...");

            // Create the configuration (normally @Bean methods)
            SecurityConfig config = new SecurityConfig();
            AIMClient aimClient = config.aimClient();
            config.configureAspect();

            System.out.println("   AIM SDK configured!");
            System.out.println("   Agent: " + aimClient.getAgentName());

            // Create services (normally @Autowired)
            CustomerService customerService = new CustomerService();
            OrderService orderService = new OrderService();

            // Simulate controller calls
            System.out.println("\n2. Simulating API calls...\n");

            // GET /api/customers/123
            System.out.println("GET /api/customers/123");
            try {
                Map<String, Object> customer = customerService.getCustomer("123");
                System.out.println("   Response: " + customer);
            } catch (ActionDeniedException e) {
                System.out.println("   403 Forbidden: " + e.getMessage());
            }

            // POST /api/customers
            System.out.println("\nPOST /api/customers");
            try {
                Map<String, Object> newCustomer = customerService.createCustomer(
                        "John Doe", "john@example.com"
                );
                System.out.println("   Response: " + newCustomer);
            } catch (ActionDeniedException e) {
                System.out.println("   403 Forbidden: " + e.getMessage());
            }

            // POST /api/orders
            System.out.println("\nPOST /api/orders");
            try {
                Map<String, Object> order = orderService.createOrder("123", Arrays.asList("item1", "item2"));
                System.out.println("   Response: " + order);
            } catch (ActionDeniedException e) {
                System.out.println("   403 Forbidden: " + e.getMessage());
            }

            // DELETE /api/customers/123
            System.out.println("\nDELETE /api/customers/123");
            try {
                customerService.deleteCustomer("123");
                System.out.println("   Response: 204 No Content");
            } catch (ActionDeniedException e) {
                System.out.println("   403 Forbidden (expected for critical): " + e.getMessage());
            }

            // Clean up
            aimClient.close();
            System.out.println("\n=== Example completed ===");

        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
            e.printStackTrace();
        }
    }

    // ===========================================
    // Spring Configuration (normally @Configuration)
    // ===========================================

    /**
     * Spring Boot configuration for AIM SDK.
     *
     * <p>In a real application, this would be:</p>
     * <pre>{@code
     * @Configuration
     * @EnableAspectJAutoProxy
     * public class SecurityConfig {
     *
     *     @Bean
     *     public AIMClient aimClient() {
     *         return AIMClient.secure("spring-boot-agent");
     *     }
     *
     *     @PostConstruct
     *     public void configureAspect() {
     *         SecureActionAspect.setClient(aimClient());
     *     }
     * }
     * }</pre>
     */
    public static class SecurityConfig {

        private AIMClient client;

        // @Bean
        public AIMClient aimClient() {
            this.client = AIMClient.secure(
                    "spring-boot-agent",
                    Arrays.asList(
                            "customer:read", "customer:write", "customer:delete",
                            "order:read", "order:write"
                    ),
                    null
            );
            return this.client;
        }

        // @PostConstruct
        public void configureAspect() {
            SecureActionAspect.setClient(this.client);
        }
    }

    // ===========================================
    // Service Layer (normally @Service)
    // ===========================================

    /**
     * Customer service with secured methods.
     *
     * <p>In a real application:</p>
     * <pre>{@code
     * @Service
     * public class CustomerService {
     *     @Autowired
     *     private CustomerRepository customerRepository;
     *     ...
     * }
     * }</pre>
     */
    public static class CustomerService {

        @SecureAction(capability = "customer:read", resource = "customers")
        public Map<String, Object> getCustomer(String id) {
            System.out.println("   [CustomerService] Getting customer: " + id);
            Map<String, Object> customer = new HashMap<>();
            customer.put("id", id);
            customer.put("name", "Jane Doe");
            customer.put("email", "jane@example.com");
            return customer;
        }

        @SecureAction(
                capability = "customer:write",
                resource = "customers",
                riskLevel = RiskLevel.MEDIUM
        )
        public Map<String, Object> createCustomer(String name, String email) {
            System.out.println("   [CustomerService] Creating customer: " + name);
            Map<String, Object> customer = new HashMap<>();
            customer.put("id", "new-" + System.currentTimeMillis());
            customer.put("name", name);
            customer.put("email", email);
            return customer;
        }

        @SecureAction(
                capability = "customer:delete",
                resource = "customers",
                riskLevel = RiskLevel.CRITICAL,
                jitAccess = true,
                jitTimeout = 300
        )
        public void deleteCustomer(String id) {
            System.out.println("   [CustomerService] Deleting customer: " + id);
            // Requires admin approval
        }
    }

    /**
     * Order service with secured methods.
     */
    public static class OrderService {

        @SecureAction(capability = "order:read", resource = "orders")
        public Map<String, Object> getOrder(String id) {
            System.out.println("   [OrderService] Getting order: " + id);
            Map<String, Object> order = new HashMap<>();
            order.put("id", id);
            order.put("status", "pending");
            return order;
        }

        @SecureAction(
                capability = "order:write",
                resource = "orders",
                riskLevel = RiskLevel.MEDIUM
        )
        public Map<String, Object> createOrder(String customerId, java.util.List<String> items) {
            System.out.println("   [OrderService] Creating order for customer: " + customerId);
            Map<String, Object> order = new HashMap<>();
            order.put("id", "order-" + System.currentTimeMillis());
            order.put("customerId", customerId);
            order.put("items", items);
            order.put("status", "created");
            return order;
        }
    }
}
