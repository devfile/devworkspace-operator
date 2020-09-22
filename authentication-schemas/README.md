This documents describes the alternatives which could be implemented for DevWorkspace endpoints authentication

# Che Workspaces Servers Authentication

Che Workspace Servers supports SSO(Single Sign-One) which means - user authenticate once on the main host and then user is automatically authenticated on workspaces hosts with loader.html + Che JWT proxy which do the SSO magic.

![](jwtproxy-current.png)

# DevWorkspaces Servers Authentication with JWTProxy + "AuthBridge" for SSO

![](jwtproxy+auth_bridge-next.png)

It's the mainly the same schema as Che Workspaces have difference here that DevWorkspace Operator should deploy additional component "AuthBridge" which will authorize requests and provide SSO.

# DevWorkspaces Servers Authentication with OpenID

![](openid-next.png)

Pros:
- no additional component is required;

Cons:
- probably different clients should be registered on OpenID provider side with workspaces specific endpoints;
- users should authenticate separately for each workspace. User will be asked if they rely that particular domain;
* we should be able to run everything on one host, and SSO could be implemented but that's not safe to do without additional authorization since OpenID token will be sent to workspace which may be fake just to steal token.
