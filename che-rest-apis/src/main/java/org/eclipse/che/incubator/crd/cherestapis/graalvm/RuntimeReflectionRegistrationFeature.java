package org.eclipse.che.incubator.crd.cherestapis.graalvm;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.GenericArrayType;
import java.lang.reflect.Method;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

import com.google.common.collect.Streams;
import com.oracle.svm.core.annotate.AutomaticFeature;

import org.graalvm.nativeimage.Feature;
import org.graalvm.nativeimage.RuntimeReflection;
import org.joda.time.DateTime;
import org.reflections.Reflections;
import org.reflections.scanners.SubTypesScanner;

import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.KubernetesList;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.internal.KubernetesDeserializer;
import io.fabric8.openshift.api.model.DeploymentConfig;
import io.fabric8.openshift.api.model.Route;
import io.fabric8.openshift.api.model.Template;
import io.kubernetes.client.custom.IntOrString;
import io.kubernetes.client.models.V1ClientIPConfig;
import io.kubernetes.client.models.V1Initializer;
import io.kubernetes.client.models.V1Initializers;
import io.kubernetes.client.models.V1ListMeta;
import io.kubernetes.client.models.V1LoadBalancerIngress;
import io.kubernetes.client.models.V1LoadBalancerStatus;
import io.kubernetes.client.models.V1ObjectMeta;
import io.kubernetes.client.models.V1OwnerReference;
import io.kubernetes.client.models.V1Service;
import io.kubernetes.client.models.V1ServiceList;
import io.kubernetes.client.models.V1ServicePort;
import io.kubernetes.client.models.V1ServiceSpec;
import io.kubernetes.client.models.V1ServiceStatus;
import io.kubernetes.client.models.V1SessionAffinityConfig;
import io.kubernetes.client.models.V1beta1HTTPIngressPath;
import io.kubernetes.client.models.V1beta1HTTPIngressRuleValue;
import io.kubernetes.client.models.V1beta1Ingress;
import io.kubernetes.client.models.V1beta1IngressBackend;
import io.kubernetes.client.models.V1beta1IngressList;
import io.kubernetes.client.models.V1beta1IngressRule;
import io.kubernetes.client.models.V1beta1IngressSpec;
import io.kubernetes.client.models.V1beta1IngressStatus;
import io.kubernetes.client.models.V1beta1IngressTLS;;

@AutomaticFeature
class RuntimeReflectionRegistrationFeature implements Feature {

  public void beforeAnalysis(BeforeAnalysisAccess access) {
    System.out.println("Eclipse Che compatibility layer for GraalVM native image generation");

    for (String prefix : Arrays.asList(
      "org.eclipse.che.api.core.rest.shared",
      "org.eclipse.che.api.core.rest.shared.dto",
      "org.eclipse.che.api.core.server.dto",
      "org.eclipse.che.api.workspace.shared.dto",
      "org.eclipse.che.api.workspace.server.dto",
      "org.eclipse.che.api.core.model.workspace",
      "org.eclipse.che.api.devfile.model"
      )) {
      Reflections reflections = new Reflections(prefix, new SubTypesScanner(false));
      Streams.concat(
        reflections.getSubTypesOf(Object.class).stream(),
        reflections.getSubTypesOf(Enum.class).stream()
      ).forEach(this::registerFully);
      registerFully(V1ServiceList.class);
      registerFully(V1Service.class);
      registerFully(V1beta1IngressList.class);
      registerFully(V1beta1Ingress.class);
      registerFully(V1ListMeta.class);
      registerFully(V1ObjectMeta.class);
      registerFully(V1ServiceSpec.class);
      registerFully(V1ServiceStatus.class);
      registerFully(V1ServicePort.class);
      registerFully(V1SessionAffinityConfig.class);
      registerFully(V1ClientIPConfig.class);
      registerFully(V1LoadBalancerStatus.class);
      registerFully(V1LoadBalancerIngress.class);
      registerFully(V1beta1IngressSpec.class);
      registerFully(V1beta1IngressBackend.class);
      registerFully(V1beta1IngressRule.class);
      registerFully(V1beta1HTTPIngressPath.class);
      registerFully(V1beta1IngressTLS.class);
      registerFully(IntOrString.class);
      registerFully(V1beta1HTTPIngressRuleValue.class);
      registerFully(V1beta1IngressStatus.class);
      registerFully(DateTime.class);
      registerFully(V1Initializers.class);
      registerFully(V1Initializer.class);
      registerFully(V1OwnerReference.class);

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