package org.eclipse.che.incubator.crd.cherestapis;

import io.kubernetes.client.ApiException;
import io.quarkus.test.common.QuarkusTestResource;
import io.quarkus.test.junit.QuarkusTest;

import org.apache.commons.io.IOUtils;
import org.eclipse.che.api.core.ServerException;
import org.eclipse.che.api.core.ValidationException;
import org.eclipse.che.api.workspace.server.devfile.exception.DevfileException;
import org.eclipse.che.api.workspace.server.model.impl.devfile.DevfileImpl;
import org.eclipse.che.api.workspace.shared.dto.WorkspaceDto;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static io.restassured.RestAssured.given;

import java.io.IOException;
import java.io.InputStream;
import java.io.StringReader;

import javax.inject.Inject;

@QuarkusTest
public class ApiServiceTest {

    @Inject
    ApiService service;

    @BeforeEach
    public void init() {
        service.init();
    }

    @Test
    public void parseDevfile()
            throws IOException, ServerException, DevfileException, ValidationException, ApiException {
        InputStream stream = this.getClass().getResourceAsStream("devfiles/petclinic-sample.yaml");
        String devfileYaml = IOUtils.toString(stream);
        DevfileImpl devfile = service.parseDevFile(devfileYaml);
        WorkspaceDto workspace = service.convertToWorkspace(devfile, null);
        Assertions.assertEquals(workspace.getConfig().getName(), "petclinic");
    }
}