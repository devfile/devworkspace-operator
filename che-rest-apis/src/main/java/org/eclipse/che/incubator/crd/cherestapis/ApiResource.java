package org.eclipse.che.incubator.crd.cherestapis;

import javax.inject.Inject;
import javax.ws.rs.BadRequestException;
import javax.ws.rs.ForbiddenException;
import javax.ws.rs.GET;
import javax.ws.rs.NotFoundException;
import javax.ws.rs.Path;
import javax.ws.rs.PathParam;
import javax.ws.rs.Produces;
import javax.ws.rs.QueryParam;
import javax.ws.rs.ServerErrorException;
import javax.ws.rs.core.MediaType;

import org.eclipse.che.api.workspace.shared.dto.WorkspaceDto;

@Path("/api")
public class ApiResource {

    @Inject
    ApiService apiService;

    @GET
    @Path("workspace/{key:.*}")
    @Produces(MediaType.APPLICATION_JSON)
    public WorkspaceDto getByKey(@PathParam("key") String key,
            @QueryParam("includeInternalServers") String includeInternalServers)
            throws NotFoundException, ServerErrorException, ForbiddenException, BadRequestException {
        return apiService.getWorkspace(key);
    }
}