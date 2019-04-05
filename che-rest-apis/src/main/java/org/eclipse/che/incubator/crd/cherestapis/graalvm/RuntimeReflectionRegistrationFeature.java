package org.eclipse.che.incubator.crd.cherestapis.graalvm;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.GenericArrayType;
import java.lang.reflect.Method;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.HashSet;
import java.util.Set;

import com.oracle.svm.core.annotate.AutomaticFeature;

import org.eclipse.che.api.workspace.server.dto.DtoServerImpls.WorkspaceDtoImpl;
import org.eclipse.che.api.workspace.server.model.impl.devfile.DevfileImpl;
import org.graalvm.nativeimage.Feature;
import org.graalvm.nativeimage.RuntimeReflection;

import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.KubernetesList;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.internal.KubernetesDeserializer;
import io.fabric8.openshift.api.model.DeploymentConfig;
import io.fabric8.openshift.api.model.Route;
import io.fabric8.openshift.api.model.Template;
import io.kubernetes.client.models.V1ServiceList;
import io.kubernetes.client.models.V1beta1IngressList;

@AutomaticFeature
class RuntimeReflectionRegistrationFeature implements Feature {

  public void beforeAnalysis(BeforeAnalysisAccess access) {
    System.out.println("Eclipse Che compatibility layer for GraalVM native image generation");

    registerFully(DevfileImpl.class);
    registerFully(WorkspaceDtoImpl.class);

    registerFully(V1ServiceList.class);
    registerFully(V1beta1IngressList.class);
    registerFully(KubernetesDeserializer.class);
    registerFully(KubernetesList.class);
    registerFully(Pod.class);
    registerFully(Service.class);
    registerFully(Template.class);
    registerFully(Route.class);
    registerFully(Deployment.class);
    registerFully(DeploymentConfig.class);
    registerFully(ConfigMap.class);
  }

  private Set<Class<?>> classesAlreadyRegistered = new HashSet<>();
  private Set<Type> typesAlreadyRegistered = new HashSet<>();

  private void registerFully(Type type) {
    if (typesAlreadyRegistered.contains(type)) {
      return;
    }
    typesAlreadyRegistered.add(type);
    if (type instanceof ParameterizedType) {
      ParameterizedType parameterizedType = (ParameterizedType) type;
      registerFully(parameterizedType.getRawType());
      for (Type paramType : parameterizedType.getActualTypeArguments()) {
        registerFully(paramType);
      }
    } else if (type instanceof GenericArrayType) {
      GenericArrayType genericArrayType = (GenericArrayType) type;
      registerFully(genericArrayType.getGenericComponentType());
    }
    else if (type instanceof Class<?>) {
      registerFully((Class<?>) type);
    }
  }

  private void registerFully(Class<?> clazz) {
    if (classesAlreadyRegistered.contains(clazz)) {
      return;
    }
    if (clazz.getPackage() == null || clazz.getPackage().getName() == null || clazz.getPackage().getName().startsWith("java")) {
      return;
    }
    System.out.println("    =>  Registering class: " + clazz.getName());
    RuntimeReflection.register(clazz);
    classesAlreadyRegistered.add(clazz);
    for (Constructor<?> constructor : clazz.getDeclaredConstructors()) {
      RuntimeReflection.register(constructor);
    }
    for (Method method : clazz.getDeclaredMethods()) {
      RuntimeReflection.register(method);
    }
    for (Field field : clazz.getDeclaredFields()) {
      RuntimeReflection.register(true, field);
      registerFully(field.getGenericType());
    }
    for (Class<?> memberClass : clazz.getDeclaredClasses()) {
      registerFully(memberClass);
    }
    Class<?> superClass = clazz.getSuperclass();
    if (superClass != null) {
      registerFully(superClass);
    }
    Class<?> enclosingClass = clazz.getEnclosingClass();
    if (enclosingClass != null) {
      registerFully(enclosingClass);
    }
  }
}