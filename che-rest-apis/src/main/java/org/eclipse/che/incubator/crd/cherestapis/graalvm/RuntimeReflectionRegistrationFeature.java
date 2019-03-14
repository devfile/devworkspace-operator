package org.eclipse.che.incubator.crd.cherestapis.graalvm;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.Arrays;

import com.google.common.collect.Streams;
import com.oracle.svm.core.annotate.AutomaticFeature;

import org.eclipse.che.commons.logback.EnvironmentVariablesLogLevelPropagator;
import org.graalvm.nativeimage.Feature;
import org.graalvm.nativeimage.RuntimeReflection;
import org.joda.time.DateTime;
import org.reflections.Reflections;
import org.reflections.scanners.SubTypesScanner;

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
      "org.eclipse.che.api.devfile.model")) {
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
    }
  }

  private void registerFully(Class<?> clazz) {
    System.out.println("    =>  Registering class:" + clazz.getSimpleName());
    RuntimeReflection.register(clazz);
    for (Constructor<?> constructor : clazz.getDeclaredConstructors()) {
      RuntimeReflection.register(constructor);
    }
    for (Method method : clazz.getDeclaredMethods()) {
      RuntimeReflection.register(method);
    }
    for (Field field : clazz.getDeclaredFields()) {
      RuntimeReflection.register(true, field);
    }
    for (Class<?> memberClass : clazz.getDeclaredClasses()) {
      registerFully(memberClass);
    }
  }

  private void registerFully(String className) {
    try {
      Class<?> clazz = Class.forName(className);
      registerFully(clazz);
    } catch(ClassNotFoundException e) {
      throw new RuntimeException(e);
    }
  }
}