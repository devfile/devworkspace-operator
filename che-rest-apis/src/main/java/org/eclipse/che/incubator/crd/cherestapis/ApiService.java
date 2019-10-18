package org.eclipse.che.incubator.crd.cherestapis;

import java.io.IOException;
import java.util.Collections;
import java.util.Map;

import javax.enterprise.context.ApplicationScoped;
import javax.enterprise.event.Observes;
import javax.inject.Inject;
import javax.ws.rs.NotFoundException;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.google.common.collect.ImmutableMap;

import org.eclipse.che.account.spi.AccountImpl;
import org.eclipse.che.api.core.ServerException;
import org.eclipse.che.api.core.ValidationException;
import org.eclipse.che.api.core.model.workspace.Runtime;
import org.eclipse.che.api.core.model.workspace.WorkspaceStatus;
import org.eclipse.che.api.workspace.server.devfile.exception.DevfileException;
import org.eclipse.che.api.workspace.server.devfile.exception.DevfileFormatException;
import org.eclipse.che.api.workspace.server.devfile.validator.DevfileIntegrityValidator;
import org.eclipse.che.api.workspace.server.dto.DtoServerImpls.RuntimeDtoImpl;
import org.eclipse.che.api.workspace.server.DtoConverter;
import org.eclipse.che.api.workspace.server.model.impl.WorkspaceImpl;
import org.eclipse.che.api.workspace.shared.dto.RuntimeDto;
import org.eclipse.che.api.workspace.shared.dto.WorkspaceDto;
import org.eclipse.che.dto.server.DtoFactory;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import org.eclipse.che.api.workspace.server.model.impl.devfile.DevfileImpl;

import io.kubernetes.client.ApiClient;
import io.kubernetes.client.ApiException;
import io.kubernetes.client.Configuration;
import io.kubernetes.client.apis.CustomObjectsApi;
import io.kubernetes.client.util.Config;
import io.quarkus.runtime.StartupEvent;

@ApplicationScoped
public class ApiService {
    private static final Logger LOGGER = LoggerFactory.getLogger("ApiService");

    @Inject
    @ConfigProperty(name = "che.workspace.name")
    String workspaceName;

    @Inject
    @ConfigProperty(name = "che.workspace.id")
    String workspaceId;

    @Inject
    @ConfigProperty(name = "che.workspace.namespace")
    String workspaceNamespace;

    @Inject
    @ConfigProperty(name = "che.workspace.crd.version", defaultValue = "v1alpha1")
    String workspaceCrdVersion;

    private ObjectMapper yamlObjectMapper = new ObjectMapper(new YAMLFactory());
    private ObjectMapper jsonObjectMapper = new ObjectMapper(new JsonFactory());
    private DevfileIntegrityValidator devfileIntegrityValidator = null;

    @SuppressWarnings("unchecked")
    private Map<String, Object> asMap(Object obj) {
        return (Map<String, Object>) obj;
    }

    public void onStart(@Observes StartupEvent ev) {
        LOGGER.info("Loading SunEC library");
        try {
            System.loadLibrary("sunec");
        } catch (Throwable t) {
            if (!t.getMessage().contains("already loaded")) {
                LOGGER.error("Error while loading the Java `sunec` dynamic library", t);
                throw t;
            }
        }

        try {
            if (workspaceId == null) {
                throw new RuntimeException("The CHE_WORKSPACE_ID environment variable should be set");
            }
            if (workspaceNamespace == null) {
                throw new RuntimeException("The CHE_WORKSPACE_NAMESPACE environment variable should be set");
            }
            if (workspaceName == null) {
                throw new RuntimeException("The CHE_WORKSPACE_NAME environment variable should be set");
            }

            LOGGER.info("Workspace Id: {}", workspaceId);
            LOGGER.info("Workspace Name: {}", workspaceName);

            init();
        } catch (RuntimeException e) {
            LOGGER.error("Che Api Service cannot start", e);
            throw e;
        }
    }

    public WorkspaceDto getWorkspace(String workspaceId) {
        LOGGER.info("Getting workspace {} {}", workspaceId, this.workspaceId);
        if (!this.workspaceId.equals(workspaceId)) {
            String message = "The workspace " + workspaceId + " is not found (current workspace is " + this.workspaceId
                    + ")";
            LOGGER.error(message);
            throw new NotFoundException(message);
        }

        String devfileYaml = null;
        String runtimeAnnotation = null;
        try {
            Map<String, Object> workspaceCustomResource = retrieveWorkspaceCustomResource();
            if (workspaceCustomResource != null) {
                Map<String, Object> status = asMap(workspaceCustomResource.get("status"));
                if (status != null) {
                    Map<String, Object> additionalFields = asMap(status.get("additionalFields"));
                    if (additionalFields != null) {
                        runtimeAnnotation = (String) additionalFields.get("org.eclipse.che.workspace/runtime");
                    }
                }
                devfileYaml = readDevfileFromWorkspaceCustomResource(workspaceCustomResource);
            }
        } catch (ApiException e) {
            throw new RuntimeException("Problem while retrieving the Workspace custom resource", e);
        } catch (JsonProcessingException e) {
            throw new RuntimeException("The devfile is not valid yaml", e);
        }
        if (devfileYaml == null) {
            throw new RuntimeException("The Workspace custom resource was not found");
        }

        DevfileImpl devfileObj;
        try {
            devfileObj = parseDevFile(devfileYaml);
        } catch (IOException e) {
            throw new RuntimeException("The devfile could not be parsed correcly: " + devfileYaml, e);
        }
        
        Runtime runtimeObj = null;
        if (runtimeAnnotation != null) {
            try {
                runtimeObj = parseRuntime(runtimeAnnotation);
            } catch (IOException e) {
                throw new RuntimeException("The devfile could not be parsed correcly: " + devfileYaml, e);
            }
        }

        LOGGER.info("Convert to workspace");

        try {
            return convertToWorkspace(devfileObj, runtimeObj);
        } catch (ServerException | DevfileException | ValidationException e) {
            throw new RuntimeException("The devfile could not be converted correcly to a workspace: " + devfileObj, e);
        } catch (ApiException e) {
            throw new RuntimeException("Problem while retrieving the Workspace runtime information from K8s objects",
                    e);
        }
    }

    private String readDevfileFromWorkspaceCustomResource(Map<String, Object> customResource) throws ApiException, JsonProcessingException {
        if (customResource == null) {
            return null;
        }
        Map<String, Object> devfileMap = asMap(asMap(customResource.get("spec")).get("devfile"));
        if (devfileMap == null) {
            return null;
        }
        return yamlObjectMapper.writeValueAsString(devfileMap);
    }

    private Map<String, Object> retrieveWorkspaceCustomResource() throws ApiException {
        return asMap(new CustomObjectsApi().getNamespacedCustomObject("workspace.che.eclipse.org", workspaceCrdVersion, workspaceNamespace,
                "workspaces", workspaceName));
    }

    DevfileImpl parseDevFile(String devfileYaml) throws JsonProcessingException, IOException {
        LOGGER.info("Devfile content for workspace {}: {}", workspaceName, devfileYaml);
        DevfileImpl devfileObj = yamlObjectMapper.treeToValue(yamlObjectMapper.readTree(devfileYaml), DevfileImpl.class);
        return devfileObj;
    }

    RuntimeDto parseRuntime(String runtimeJson) throws JsonProcessingException, IOException {
        LOGGER.info("Runtime content for workspace {}: {}", workspaceName, runtimeJson);
        RuntimeDto runtimeObj = DtoFactory.getInstance().createDtoFromJson(runtimeJson, RuntimeDto.class);
        return runtimeObj;
    }

    WorkspaceDto convertToWorkspace(DevfileImpl devfileObj, Runtime runtimeObj) throws DevfileException, ServerException, ValidationException, ApiException {
        LOGGER.info("validateDevfile");
        try {
            devfileIntegrityValidator.validateDevfile(devfileObj);
        } catch(DevfileFormatException e) {
            LOGGER.warn("Validation of the devfile failed", e);
        }

        LOGGER.info(" WorkspaceImpl.builder().build()");
        WorkspaceImpl workspace = WorkspaceImpl.builder()
            .setId(workspaceId)
            .setConfig(null)
            .setDevfile(devfileObj)
            .setAccount(new AccountImpl("anonymous", "anonymous", "anonymous"))
            .setAttributes(Collections.emptyMap())
            .setTemporary(false)
            .setRuntime(runtimeObj)
            .setStatus(WorkspaceStatus.RUNNING)
            .build();

        return DtoConverter.asDto(workspace);
    }

    void init() {
        devfileIntegrityValidator = new DevfileIntegrityValidator(ImmutableMap.of());
        try {
            ApiClient client = Config.defaultClient();
            Configuration.setDefaultApiClient(client);
        } catch(IOException e) {
            throw new RuntimeException("Kubernetes client cannot be created", e);
        }
    }

}

