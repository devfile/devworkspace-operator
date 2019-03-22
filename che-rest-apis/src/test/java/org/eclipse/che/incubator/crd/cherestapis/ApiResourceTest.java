package org.eclipse.che.incubator.crd.cherestapis;

import io.quarkus.test.junit.QuarkusTest;
import org.junit.jupiter.api.Test;

import static io.restassured.RestAssured.given;

@QuarkusTest
public class ApiResourceTest {

    @Test
    public void testHelloEndpoint() {
        given()
          .when().get("/api/workspace/unknownId")
          .then()
             .statusCode(404);
    }
}