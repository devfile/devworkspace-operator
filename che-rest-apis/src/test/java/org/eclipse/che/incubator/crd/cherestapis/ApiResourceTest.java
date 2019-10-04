package org.eclipse.che.incubator.crd.cherestapis;

import io.quarkus.test.junit.QuarkusTest;

import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

import static io.restassured.RestAssured.given;

@QuarkusTest
public class ApiResourceTest {

    @Disabled
    @Test
    public void testHelloEndpoint() {
        given()
          .when().get("/api/workspace/unkn")
          .then()
             .statusCode(404);
    }
}