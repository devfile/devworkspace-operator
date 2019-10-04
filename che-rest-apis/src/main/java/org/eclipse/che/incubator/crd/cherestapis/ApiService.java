package org.eclipse.che.incubator.crd.cherestapis;

import java.io.IOException;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;

import javax.enterprise.context.ApplicationScoped;
import javax.enterprise.event.Observes;
import javax.inject.Inject;
import javax.ws.rs.NotFoundException;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.google.common.collect.ImmutableMap;

import org.eclipse.che.account.spi.AccountImpl;
import org.eclipse.che.api.core.ServerException;
import org.eclipse.che.api.core.ValidationException;
import org.eclipse.che.api.core.model.workspace.Runtime;
import org.eclipse.che.api.core.model.workspace.WorkspaceStatus;
import org.eclipse.che.api.core.model.workspace.runtime.MachineStatus;
import org.eclipse.che.api.core.model.workspace.runtime.ServerStatus;
import org.eclipse.che.api.workspace.server.devfile.exception.DevfileException;
import org.eclipse.che.api.workspace.server.devfile.exception.DevfileFormatException;
import org.eclipse.che.api.workspace.server.devfile.validator.DevfileIntegrityValidator;
import org.eclipse.che.api.workspace.server.DtoConverter;
import org.eclipse.che.api.workspace.server.model.impl.MachineImpl;
import org.eclipse.che.api.workspace.server.model.impl.RuntimeImpl;
import org.eclipse.che.api.workspace.server.model.impl.ServerImpl;
import org.eclipse.che.api.workspace.server.model.impl.WorkspaceImpl;
import org.eclipse.che.api.workspace.shared.dto.WorkspaceDto;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import org.eclipse.che.api.workspace.server.model.impl.devfile.DevfileImpl;

import io.kubernetes.client.ApiClient;
import io.kubernetes.client.ApiException;
import io.kubernetes.client.Configuration;
import io.kubernetes.client.apis.CoreV1Api;
import io.kubernetes.client.apis.CustomObjectsApi;
import io.kubernetes.client.apis.ExtensionsV1beta1Api;
import io.kubernetes.client.models.V1Service;
import io.kubernetes.client.models.V1ServiceList;
import io.kubernetes.client.models.V1beta1Ingress;
import io.kubernetes.client.models.V1beta1IngressList;
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
            if (! t.getMessage().contains("already loaded")) {
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

        String devfileYaml;
        try {
            devfileYaml = readDevfileFromWorkspaceCustomResource();
        } catch(ApiException e) {
            throw new RuntimeException("Problem while retrieving the Workspace custom resource", e);
        } catch(JsonProcessingException e) {
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

        LOGGER.info("Convert to workspace");

        try {
            return convertToWorkspace(devfileObj);
        } catch (ServerException | DevfileException | ValidationException e) {
            throw new RuntimeException("The devfile could not be converted correcly to a workspace: " + devfileObj, e);
        } catch(ApiException e) {
            throw new RuntimeException("Problem while retrieving the Workspace runtime information from K8s objects", e);
        }
    }

    private String readDevfileFromWorkspaceCustomResource() throws ApiException, JsonProcessingException {
        CustomObjectsApi api = new CustomObjectsApi();
        Map<String, Object> customResource = asMap(api.getNamespacedCustomObject("workspace.che.eclipse.org", workspaceCrdVersion,
                workspaceNamespace, "workspaces", workspaceName));
        if (customResource == null) {
            return null;
        }
        Map<String, Object> devfileMap = asMap(asMap(customResource.get("spec")).get("devfile"));
        if (devfileMap == null) {
            return null;
        }
        return yamlObjectMapper.writeValueAsString(devfileMap);
    }

    private String workspaceIdSelector() {
        return "che.workspace_id = " + workspaceId;
    }

    private Stream<V1Service> listWorkspaceServices() {
        CoreV1Api coreApi = new CoreV1Api();

        V1ServiceList services;
        try {
            services = coreApi.listNamespacedService(workspaceNamespace, Boolean.TRUE, null, null, null,
                    workspaceIdSelector(), 30, null, 1000, Boolean.FALSE);
            return services.getItems().stream();
        } catch (ApiException e) {
            LOGGER.error("Problem while retrieving the workspace services", e);
            return Collections.<V1Service>emptyList().stream();
        }
    }

    private Stream<V1beta1Ingress> listWorkspaceIngresses() {
        ExtensionsV1beta1Api extensionsApi = new ExtensionsV1beta1Api();
        try {
            V1beta1IngressList ingresses = extensionsApi.listNamespacedIngress(workspaceNamespace, Boolean.TRUE, null,
                    null, null, workspaceIdSelector(), 100, null, 1000, Boolean.FALSE);
            return ingresses.getItems().stream();
        } catch (ApiException e) {
            LOGGER.error("Problem while retrieving the workspace ingresses", e);
            return Collections.<V1beta1Ingress>emptyList().stream();
        }
    }

    private String serverName(V1beta1Ingress ingress) {
        return ingress.getMetadata().getName().replace("ingress-" + workspaceId + "-", "");
    }

    private String serverUrl(V1beta1Ingress ingress) {
        return ingress.getMetadata().getAnnotations().get("org.eclipse.che.server.protocol")
        + "://" + ingress.getSpec().getRules().get(0).getHost();
    }

    private Map<String, String> serverAttributes(V1beta1Ingress ingress) {
        Map<String, String> attributes = new HashMap<>();
        String attributesAnnotation = ingress.getMetadata().getAnnotations().get("org.eclipse.che.server.attributes");
        if (attributesAnnotation != null) {
            try {
                JsonNode jsonNode = jsonObjectMapper.readTree(attributesAnnotation);
                jsonNode.fields().forEachRemaining(field -> {
                    attributes.put(field.getKey(), field.getValue().asText());
                });
            } catch(IOException e) {
                LOGGER.error("Problem while parsing ingress attributes annotation for ingress" + ingress.getMetadata().getName(), e);
            } 
        }
        String portAnnotation = ingress.getMetadata().getAnnotations().get("org.eclipse.che.server.port");
        if (portAnnotation != null) {
            attributes.put("port", portAnnotation.split("/")[0]);
        }
        return attributes;
    }

    private Runtime buildRuntimeFromK8sObjects() throws ApiException {
        Map<String, MachineImpl> machines = listWorkspaceServices()
        .filter(service -> service.getMetadata().getAnnotations() != null&&
                            service.getMetadata().getAnnotations().containsKey("org.eclipse.che.machine.name"))
        .collect(ImmutableMap.toImmutableMap(
            service -> service.getMetadata().getAnnotations().get("org.eclipse.che.machine.name"),
            service -> new MachineImpl(
                    service.getMetadata().getAnnotations().entrySet().stream()
                        .filter(entry -> entry.getKey().startsWith("org.eclipse.che.machine.") 
                                && ! entry.getKey().equals("org.eclipse.che.machine.name"))
                        .collect(Collectors.toMap(
                            entry -> entry.getKey().replace("org.eclipse.che.machine.", ""),
                            entry -> entry.getValue())),
                    listWorkspaceIngresses()
                        .filter(ingress -> 
                            service.getMetadata().getAnnotations().get("org.eclipse.che.machine.name").equals(
                                ingress.getMetadata().getAnnotations().get("org.eclipse.che.machine.name")))
                        .collect(ImmutableMap.toImmutableMap(
                            ingress -> serverName(ingress),
                            ingress -> new ServerImpl(serverUrl(ingress), ServerStatus.UNKNOWN, serverAttributes(ingress))
                        )),
                    MachineStatus.RUNNING
                )
        ));
        return new RuntimeImpl(
            "default",
            machines,
            "anonymous"
        );
    }

    DevfileImpl parseDevFile(String devfileYaml) throws JsonProcessingException, IOException {
        LOGGER.info("Devfile content for workspace {}: {}", workspaceName, devfileYaml);
        DevfileImpl devfileObj = yamlObjectMapper.treeToValue(yamlObjectMapper.readTree(devfileYaml), DevfileImpl.class);
        return devfileObj;
    }

    WorkspaceDto convertToWorkspace(DevfileImpl devfileObj) throws DevfileException, ServerException, ValidationException, ApiException {
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
            .setRuntime(buildRuntimeFromK8sObjects())
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

